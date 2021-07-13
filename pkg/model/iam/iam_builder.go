/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// IAM Documentation: /docs/iam_roles.md

// TODO: We have a couple different code paths until we do lifecycles, and
// TODO: when we have a cluster or refactor some s3 code.  The only code that
// TODO: is not shared by the different path is the s3 / state store stuff.

// TODO: Initial work has been done to lock down IAM actions based on resources
// TODO: and condition keys, but this can be extended further (with thorough testing).

package iam

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/apis/kops/model"
	"k8s.io/kops/pkg/util/stringorslice"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/awstasks"
	"k8s.io/kops/util/pkg/vfs"
)

// PolicyDefaultVersion is the default version included in all policy documents
const PolicyDefaultVersion = "2012-10-17"

// Policy Struct is a collection of fields that form a valid AWS policy document
type Policy struct {
	clusterName         string
	unconditionalAction sets.String
	clusterTaggedAction sets.String
	Statement           []*Statement
	Version             string
}

// AsJSON converts the policy document to JSON format (parsable by AWS)
func (p *Policy) AsJSON() (string, error) {
	if len(p.unconditionalAction) > 0 {
		p.Statement = append(p.Statement, &Statement{
			Effect:   StatementEffectAllow,
			Action:   stringorslice.Of(p.unconditionalAction.List()...),
			Resource: stringorslice.String("*"),
		})
	}
	if len(p.clusterTaggedAction) > 0 {
		p.Statement = append(p.Statement, &Statement{
			Effect:   StatementEffectAllow,
			Action:   stringorslice.Of(p.clusterTaggedAction.List()...),
			Resource: stringorslice.String("*"),
			Condition: Condition{
				"StringEquals": map[string]string{
					"aws:ResourceTag/KubernetesCluster": p.clusterName,
				},
			},
		})
	}

	j, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshaling policy to JSON: %v", err)
	}
	return string(j), nil
}

// StatementEffect is required and specifies what type of access the statement results in
type StatementEffect string

// StatementEffectAllow allows access for the given resources in the statement (based on conditions)
const StatementEffectAllow StatementEffect = "Allow"

// StatementEffectDeny allows access for the given resources in the statement (based on conditions)
const StatementEffectDeny StatementEffect = "Deny"

// Condition is a map of Conditions to be evaluated for a given IAM Statement
type Condition map[string]interface{}

// Statement is an AWS IAM Policy Statement Object:
// http://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements.html#Statement
type Statement struct {
	Effect    StatementEffect
	Principal Principal
	Action    stringorslice.StringOrSlice
	Resource  stringorslice.StringOrSlice
	Condition Condition
}

type jsonWriter struct {
	w   io.Writer
	err error
}

func (j *jsonWriter) Error() error {
	return j.err
}

func (j *jsonWriter) WriteLiteral(b []byte) {
	if j.err != nil {
		return
	}
	_, err := j.w.Write(b)
	if err != nil {
		j.err = err
	}
}

func (j *jsonWriter) StartObject() {
	j.WriteLiteral([]byte("{"))
}

func (j *jsonWriter) EndObject() {
	j.WriteLiteral([]byte("}"))
}

func (j *jsonWriter) Comma() {
	j.WriteLiteral([]byte(","))
}

func (j *jsonWriter) Field(s string) {
	if j.err != nil {
		return
	}
	b, err := json.Marshal(s)
	if err != nil {
		j.err = err
		return
	}
	j.WriteLiteral(b)
	j.WriteLiteral([]byte(": "))
}

func (j *jsonWriter) Marshal(v interface{}) {
	if j.err != nil {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		j.err = err
		return
	}
	j.WriteLiteral(b)
}

// MarshalJSON formats the IAM statement for the AWS IAM restrictions.
// For example, `Resource: []` is not allowed, but golang would force us to use pointers.
func (s *Statement) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer

	jw := &jsonWriter{w: &b}
	jw.StartObject()

	if !s.Action.IsEmpty() {
		jw.Field("Action")
		jw.Marshal(s.Action)
		jw.Comma()
	}

	if len(s.Condition) != 0 {
		jw.Field("Condition")
		jw.Marshal(s.Condition)
		jw.Comma()
	}

	jw.Field("Effect")
	jw.Marshal(s.Effect)

	if !s.Principal.IsEmpty() {
		jw.Comma()
		jw.Field("Principal")
		jw.Marshal(s.Principal)
	}

	if !s.Resource.IsEmpty() {
		jw.Comma()
		jw.Field("Resource")
		jw.Marshal(s.Resource)
	}

	jw.EndObject()

	return b.Bytes(), jw.Error()
}

type Principal struct {
	Federated string `json:",omitempty"`
	Service   string `json:",omitempty"`
}

func (p *Principal) IsEmpty() bool {
	return *p == Principal{}
}

// Equal compares two IAM Statements and returns a bool
// TODO: Extend to support Condition Keys
func (l *Statement) Equal(r *Statement) bool {
	if l.Effect != r.Effect {
		return false
	}
	if !l.Action.Equal(r.Action) {
		return false
	}
	if !l.Resource.Equal(r.Resource) {
		return false
	}
	return true
}

// PolicyBuilder struct defines all valid fields to be used when building the
// AWS IAM policy document for a given instance group role.
type PolicyBuilder struct {
	Cluster              *kops.Cluster
	HostedZoneID         string
	KMSKeys              []string
	Region               string
	ResourceARN          *string
	Role                 Subject
	UseServiceAccountIAM bool
}

// BuildAWSPolicy builds a set of IAM policy statements based on the
// instance group type and IAM Legacy flag within the Cluster Spec
func (b *PolicyBuilder) BuildAWSPolicy() (*Policy, error) {
	// Retrieve all the KMS Keys in use
	for _, e := range b.Cluster.Spec.EtcdClusters {
		for _, m := range e.Members {
			if m.KmsKeyId != nil {
				b.KMSKeys = append(b.KMSKeys, *m.KmsKeyId)
			}
		}
	}

	p, err := b.Role.BuildAWSPolicy(b)
	if err != nil {
		return nil, fmt.Errorf("failed to generate AWS IAM Policy: %v", err)
	}

	return p, nil
}

func NewPolicy(clusterName string) *Policy {
	p := &Policy{
		Version:             PolicyDefaultVersion,
		clusterName:         clusterName,
		unconditionalAction: sets.NewString(),
		clusterTaggedAction: sets.NewString(),
	}
	return p
}

// BuildAWSPolicy generates a custom policy for a Kubernetes master.
func (r *NodeRoleAPIServer) BuildAWSPolicy(b *PolicyBuilder) (*Policy, error) {
	p := NewPolicy(b.Cluster.GetClusterName())

	AddMasterEC2Policies(p)
	addASLifecyclePolicies(p, r.warmPool)
	addCertIAMPolicies(p)
	addKMSGenerateRandomPolicies(p)

	var err error
	if p, err = b.AddS3Permissions(p); err != nil {
		return nil, fmt.Errorf("failed to generate AWS IAM S3 access statements: %v", err)
	}

	if b.KMSKeys != nil && len(b.KMSKeys) != 0 {
		addKMSIAMPolicies(p, stringorslice.Slice(b.KMSKeys))
	}

	if b.Cluster.Spec.IAM.AllowContainerRegistry {
		addECRPermissions(p)
	}

	if b.Cluster.Spec.Networking != nil && b.Cluster.Spec.Networking.AmazonVPC != nil {
		addAmazonVPCCNIPermissions(p, b.IAMPrefix())
	}

	if b.Cluster.Spec.Networking != nil && b.Cluster.Spec.Networking.LyftVPC != nil {
		addLyftVPCPermissions(p)
	}

	if b.Cluster.Spec.Networking != nil && b.Cluster.Spec.Networking.Cilium != nil && b.Cluster.Spec.Networking.Cilium.Ipam == kops.CiliumIpamEni {
		addCiliumEniPermissions(p)
	}

	if b.Cluster.Spec.Networking != nil && b.Cluster.Spec.Networking.Calico != nil && b.Cluster.Spec.Networking.Calico.AWSSrcDstCheck != "DoNothing" {
		addCalicoSrcDstCheckPermissions(p)
	}

	return p, nil
}

// BuildAWSPolicy generates a custom policy for a Kubernetes master.
func (r *NodeRoleMaster) BuildAWSPolicy(b *PolicyBuilder) (*Policy, error) {
	clusterName := b.Cluster.GetName()

	p := NewPolicy(clusterName)

	AddMasterEC2Policies(p)
	addASLifecyclePolicies(p, true)
	addMasterASPolicies(p)
	AddMasterELBPolicies(p)
	addCertIAMPolicies(p)
	addKMSGenerateRandomPolicies(p)

	var err error
	if p, err = b.AddS3Permissions(p); err != nil {
		return nil, fmt.Errorf("failed to generate AWS IAM S3 access statements: %v", err)
	}

	if b.KMSKeys != nil && len(b.KMSKeys) != 0 {
		addKMSIAMPolicies(p, stringorslice.Slice(b.KMSKeys))
	}

	// Protokube needs dns-controller permissions in instance role even if UseServiceAccountIAM.
	AddDNSControllerPermissions(b, p)

	if !b.UseServiceAccountIAM {
		esc := b.Cluster.Spec.SnapshotController != nil &&
			fi.BoolValue(b.Cluster.Spec.SnapshotController.Enabled)
		AddAWSEBSCSIDriverPermissions(p, esc)

		if b.Cluster.Spec.AWSLoadBalancerController != nil && fi.BoolValue(b.Cluster.Spec.AWSLoadBalancerController.Enabled) {
			AddAWSLoadbalancerControllerPermissions(p)
		}
		AddClusterAutoscalerPermissions(p)
	}

	if b.Cluster.Spec.IAM.AllowContainerRegistry {
		addECRPermissions(p)
	}

	if b.Cluster.Spec.Networking != nil && b.Cluster.Spec.Networking.AmazonVPC != nil {
		addAmazonVPCCNIPermissions(p, b.IAMPrefix())
	}

	if b.Cluster.Spec.Networking != nil && b.Cluster.Spec.Networking.LyftVPC != nil {
		addLyftVPCPermissions(p)
	}

	if b.Cluster.Spec.Networking != nil && b.Cluster.Spec.Networking.Cilium != nil && b.Cluster.Spec.Networking.Cilium.Ipam == kops.CiliumIpamEni {
		addCiliumEniPermissions(p)
	}

	if b.Cluster.Spec.Networking != nil && b.Cluster.Spec.Networking.Calico != nil && b.Cluster.Spec.Networking.Calico.AWSSrcDstCheck != "DoNothing" {
		addCalicoSrcDstCheckPermissions(p)
	}

	nth := b.Cluster.Spec.NodeTerminationHandler
	if nth != nil && fi.BoolValue(nth.Enabled) && fi.BoolValue(nth.EnableSQSTerminationDraining) {
		addNodeTerminationHandlerSQSPermissions(p)
	}

	if b.Cluster.Spec.SnapshotController != nil && fi.BoolValue(b.Cluster.Spec.SnapshotController.Enabled) {
		addSnapshotPersmissions(p)
	}
	return p, nil
}

// BuildAWSPolicy generates a custom policy for a Kubernetes node.
func (r *NodeRoleNode) BuildAWSPolicy(b *PolicyBuilder) (*Policy, error) {
	p := NewPolicy(b.Cluster.GetClusterName())

	addNodeEC2Policies(p)
	addASLifecyclePolicies(p, r.enableLifecycleHookPermissions)
	addKMSGenerateRandomPolicies(p)

	var err error
	if p, err = b.AddS3Permissions(p); err != nil {
		return nil, fmt.Errorf("failed to generate AWS IAM S3 access statements: %v", err)
	}

	if b.Cluster.Spec.IAM.AllowContainerRegistry {
		addECRPermissions(p)
	}

	if b.Cluster.Spec.Networking != nil && b.Cluster.Spec.Networking.AmazonVPC != nil {
		addAmazonVPCCNIPermissions(p, b.IAMPrefix())
	}

	if b.Cluster.Spec.Networking != nil && b.Cluster.Spec.Networking.LyftVPC != nil {
		addLyftVPCPermissions(p)
	}

	if b.Cluster.Spec.Networking != nil && b.Cluster.Spec.Networking.Calico != nil && b.Cluster.Spec.Networking.Calico.AWSSrcDstCheck != "DoNothing" {
		addCalicoSrcDstCheckPermissions(p)
	}

	return p, nil
}

// BuildAWSPolicy generates a custom policy for a bastion host.
func (r *NodeRoleBastion) BuildAWSPolicy(b *PolicyBuilder) (*Policy, error) {
	p := NewPolicy(b.Cluster.GetClusterName())

	// Bastion hosts currently don't require any specific permissions.
	// A trivial permission is granted, because empty policies are not allowed.
	p.unconditionalAction.Insert("ec2:DescribeRegions")

	return p, nil
}

// IAMPrefix returns the prefix for AWS ARNs in the current region, for use with IAM
// it is arn:aws in the default aws partition but different in other isolated or non-standard partitions
func (b *PolicyBuilder) IAMPrefix() string {
	partitions := endpoints.DefaultPartitions()
	for _, p := range partitions {
		if _, ok := p.Regions()[b.Region]; ok {
			arn := "arn:" + p.ID()
			return arn
		}
	}
	return "arn:aws"
}

// AddS3Permissions builds an IAM Policy, with statements granting tailored
// access to S3 assets, depending on the instance group or service-account role
func (b *PolicyBuilder) AddS3Permissions(p *Policy) (*Policy, error) {
	// For S3 IAM permissions we grant permissions to subtrees, so find the parents;
	// we don't need to grant mypath and mypath/child.
	var roots []string
	{
		var locations []string

		for _, p := range []string{
			b.Cluster.Spec.KeyStore,
			b.Cluster.Spec.SecretStore,
			b.Cluster.Spec.ConfigStore,
		} {
			if p == "" {
				continue
			}

			if !strings.HasSuffix(p, "/") {
				p = p + "/"
			}
			locations = append(locations, p)
		}

		for i, l := range locations {
			isTopLevel := true
			for j := range locations {
				if i == j {
					continue
				}
				if strings.HasPrefix(l, locations[j]) {
					klog.V(4).Infof("Ignoring location %q because found parent %q", l, locations[j])
					isTopLevel = false
				}
			}
			if isTopLevel {
				klog.V(4).Infof("Found root location %q", l)
				roots = append(roots, l)
			}
		}
	}

	sort.Strings(roots)

	s3Buckets := sets.NewString()

	for _, root := range roots {
		vfsPath, err := vfs.Context.BuildVfsPath(root)
		if err != nil {
			return nil, fmt.Errorf("cannot parse VFS path %q: %v", root, err)
		}

		switch path := vfsPath.(type) {
		case *vfs.S3Path:
			iamS3Path := path.Bucket() + "/" + path.Key()
			iamS3Path = strings.TrimSuffix(iamS3Path, "/")

			s3Buckets.Insert(path.Bucket())

			if err := b.buildS3GetStatements(p, iamS3Path); err != nil {
				return nil, err
			}

		case *vfs.MemFSPath:
			// Tests - we emulate the s3 permissions so that we can get an idea of the full policy

			iamS3Path := "placeholder-read-bucket/" + path.Location()
			b.buildS3GetStatements(p, iamS3Path)
			s3Buckets.Insert("placeholder-read-bucket")
		case *vfs.FSPath:
			// tests - we emulate the s3 permissions so that we can get an idea of the full policy

			iamS3path := "placeholder-read-bucket/" + strings.TrimPrefix(path.Path(), "file://")
			b.buildS3GetStatements(p, iamS3path)
			s3Buckets.Insert("placeholder-read-bucket")
		case *vfs.VaultPath:
			// Vault access needs to come from somewhere else
			klog.Warningf("ignoring valult path %q for IAM policy builder", vfsPath)
		default:
			// We could implement this approach, but it seems better to
			// get all clouds using cluster-readable storage
			return nil, fmt.Errorf("path is not cluster readable: %v", root)
		}
	}

	writeablePaths, err := WriteableVFSPaths(b.Cluster, b.Role)
	if err != nil {
		return nil, err
	}

	for _, vfsPath := range writeablePaths {
		switch path := vfsPath.(type) {
		case *vfs.S3Path:
			iamS3Path := path.Bucket() + "/" + path.Key()
			iamS3Path = strings.TrimSuffix(iamS3Path, "/")

			b.buildS3WriteStatements(p, iamS3Path)
			s3Buckets.Insert(path.Bucket())
		case *vfs.MemFSPath:
			iamS3Path := "placeholder-write-bucket/" + path.Location()
			b.buildS3WriteStatements(p, iamS3Path)
			s3Buckets.Insert("placeholder-write-bucket")
		case *vfs.FSPath:
			iamS3path := "placeholder-read-bucket/" + strings.TrimPrefix(path.Path(), "file://")
			b.buildS3WriteStatements(p, iamS3path)
			s3Buckets.Insert("placeholder-read-bucket")
		default:
			return nil, fmt.Errorf("unknown writeable path, can't apply IAM policy: %q", vfsPath)
		}
	}

	// We need some permissions on the buckets themselves
	for _, s3Bucket := range s3Buckets.List() {
		p.Statement = append(p.Statement, &Statement{
			Effect: StatementEffectAllow,
			Action: stringorslice.Of(
				"s3:GetBucketLocation",
				"s3:GetEncryptionConfiguration",
				"s3:ListBucket",
				"s3:ListBucketVersions",
			),
			Resource: stringorslice.Slice([]string{
				strings.Join([]string{b.IAMPrefix(), ":s3:::", s3Bucket}, ""),
			}),
		})
	}

	return p, nil
}

func (b *PolicyBuilder) buildS3WriteStatements(p *Policy, iamS3Path string) {
	p.Statement = append(p.Statement, &Statement{
		Effect: StatementEffectAllow,
		Action: stringorslice.Slice([]string{
			"s3:GetObject",
			"s3:DeleteObject",
			"s3:DeleteObjectVersion",
			"s3:PutObject",
		}),
		Resource: stringorslice.Of(
			strings.Join([]string{b.IAMPrefix(), ":s3:::", iamS3Path, "/*"}, ""),
		),
	})

}

func (b *PolicyBuilder) buildS3GetStatements(p *Policy, iamS3Path string) error {

	resources, err := ReadableStatePaths(b.Cluster, b.Role)
	if err != nil {
		return err
	}

	if len(resources) != 0 {
		sort.Strings(resources)

		// Add the prefix for IAM
		for i, r := range resources {
			resources[i] = b.IAMPrefix() + ":s3:::" + iamS3Path + r
		}

		p.Statement = append(p.Statement, &Statement{
			Effect:   StatementEffectAllow,
			Action:   stringorslice.Slice([]string{"s3:Get*"}),
			Resource: stringorslice.Of(resources...),
		})
	}
	return nil
}

func WriteableVFSPaths(cluster *kops.Cluster, role Subject) ([]vfs.Path, error) {
	var paths []vfs.Path

	// etcd-manager needs write permissions to the backup store
	switch role.(type) {
	case *NodeRoleMaster:
		backupStores := sets.NewString()
		for _, c := range cluster.Spec.EtcdClusters {
			if c.Backups == nil || c.Backups.BackupStore == "" || backupStores.Has(c.Backups.BackupStore) {
				continue
			}
			backupStore := c.Backups.BackupStore

			vfsPath, err := vfs.Context.BuildVfsPath(backupStore)
			if err != nil {
				return nil, fmt.Errorf("cannot parse VFS path %q: %v", backupStore, err)
			}

			paths = append(paths, vfsPath)

			backupStores.Insert(backupStore)
		}
	}

	return paths, nil
}

// ReadableStatePaths returns the file paths that should be readable in the cluster's state store "directory"
func ReadableStatePaths(cluster *kops.Cluster, role Subject) ([]string, error) {
	var paths []string

	switch role.(type) {
	case *NodeRoleMaster, *NodeRoleAPIServer:
		paths = append(paths, "/*")

	case *NodeRoleNode:
		paths = append(paths,
			"/addons/*",
			"/cluster-completed.spec",
			"/igconfig/node/*",
			"/pki/ssh/*",
			"/secrets/dockerconfig",
		)

		// Give access to keys for client certificates as needed.
		if !model.UseKopsControllerForNodeBootstrap(cluster) {
			paths = append(paths, "/pki/private/kube-proxy/*")

			if useBootstrapTokens(cluster) {
				paths = append(paths, "/pki/private/node-authorizer-client/*")
			} else {
				paths = append(paths, "/pki/private/kubelet/*")
			}

			networkingSpec := cluster.Spec.Networking

			if networkingSpec != nil {
				// @check if kuberoute is enabled and permit access to the private key
				if networkingSpec.Kuberouter != nil {
					paths = append(paths, "/pki/private/kube-router/*")
				}

				// @check if cilium is enabled as the CNI provider and permit access to the cilium etc client TLS certificate by default
				// As long as the Cilium Etcd cluster exists, we should do this
				if networkingSpec.Cilium != nil && model.UseCiliumEtcd(cluster) {
					paths = append(paths, "/pki/private/etcd-client-cilium/*")
				}
			}
		}
	}
	return paths, nil
}

// PolicyResource defines the PolicyBuilder and DNSZone to use when building the
// IAM policy document for a given instance group role
type PolicyResource struct {
	Builder *PolicyBuilder
	DNSZone *awstasks.DNSZone
}

var _ fi.Resource = &PolicyResource{}
var _ fi.HasDependencies = &PolicyResource{}

// GetDependencies adds the DNSZone task to the list of dependencies if set
func (b *PolicyResource) GetDependencies(tasks map[string]fi.Task) []fi.Task {
	var deps []fi.Task
	if b.DNSZone != nil {
		deps = append(deps, b.DNSZone)
	}
	return deps
}

// Open produces the AWS IAM policy for the given role
func (b *PolicyResource) Open() (io.Reader, error) {
	// Defensive copy before mutation
	pb := *b.Builder

	if b.DNSZone != nil {
		hostedZoneID := fi.StringValue(b.DNSZone.ZoneID)
		if hostedZoneID == "" {
			// Dependency analysis failure?
			return nil, fmt.Errorf("DNS ZoneID not set")
		}
		pb.HostedZoneID = hostedZoneID
	}

	policy, err := pb.BuildAWSPolicy()
	if err != nil {
		return nil, fmt.Errorf("error building IAM policy: %v", err)
	}
	if policy == nil {
		return bytes.NewReader([]byte{}), nil
	}
	j, err := policy.AsJSON()
	if err != nil {
		return nil, fmt.Errorf("error building IAM policy: %v", err)
	}
	return bytes.NewReader([]byte(j)), nil
}

// useBootstrapTokens check if we are using bootstrap tokens - @TODO, i don't like this we should probably pass in
// the kops model into the builder rather than duplicating the code. I'll leave for another PR
func useBootstrapTokens(cluster *kops.Cluster) bool {
	if cluster.Spec.KubeAPIServer == nil {
		return false
	}

	return fi.BoolValue(cluster.Spec.KubeAPIServer.EnableBootstrapAuthToken)
}

func addECRPermissions(p *Policy) {
	// TODO - I think we can just have GetAuthorizationToken here, as we are not
	// TODO - making any API calls except for GetAuthorizationToken.

	// We provide ECR access on the nodes (naturally), but we also provide access on the master.
	// We shouldn't be running lots of pods on the master, but it is perfectly reasonable to run
	// a private logging pod or similar.
	// At this point we allow all regions with ECR, since ECR is region specific.
	p.unconditionalAction.Insert(
		"ecr:GetAuthorizationToken",
		"ecr:BatchCheckLayerAvailability",
		"ecr:GetDownloadUrlForLayer",
		"ecr:GetRepositoryPolicy",
		"ecr:DescribeRepositories",
		"ecr:ListImages",
		"ecr:BatchGetImage",
	)
}

func addCalicoSrcDstCheckPermissions(p *Policy) {
	p.unconditionalAction.Insert(
		"ec2:DescribeInstances",
		"ec2:ModifyNetworkInterfaceAttribute",
	)
}

// AddAWSLoadbalancerControllerPermissions adds the permissions needed for the aws load balancer controller to the givnen policy
func AddAWSLoadbalancerControllerPermissions(p *Policy) {
	p.unconditionalAction.Insert(
		"ec2:DescribeAvailabilityZones",
		"ec2:DescribeNetworkInterfaces",
		"elasticloadbalancing:DescribeTags",
		"elasticloadbalancing:DescribeTargetGroupAttributes",
		"elasticloadbalancing:DescribeRules",
		"elasticloadbalancing:DescribeTargetHealth",
		"elasticloadbalancing:DescribeListenerCertificates",
		"elasticloadbalancing:CreateRule",
	)
	p.Statement = append(p.Statement,
		&Statement{
			Effect: StatementEffectAllow,
			Action: stringorslice.Of(
				"ec2:AuthorizeSecurityGroupIngress", // aws.go
				"ec2:DeleteSecurityGroup",           // aws.go
				"ec2:RevokeSecurityGroupIngress",    // aws.go

				"elasticloadbalancing:ModifyTargetGroupAttributes",
				"elasticloadbalancing:ModifyRule",
				"elasticloadbalancing:DeleteRule",

				"elasticloadbalancing:AddTags",
				"elasticloadbalancing:RemoveTags",
			),
			Resource: stringorslice.String("*"),
			Condition: Condition{
				"StringEquals": map[string]string{
					"aws:ResourceTag/elbv2.k8s.aws/cluster": p.clusterName,
				},
			},
		},
	)
}

func AddClusterAutoscalerPermissions(p *Policy) {
	p.clusterTaggedAction.Insert(
		"autoscaling:SetDesiredCapacity",
		"autoscaling:TerminateInstanceInAutoScalingGroup",
	)
	p.unconditionalAction.Insert(
		"autoscaling:DescribeAutoScalingGroups",
		"autoscaling:DescribeAutoScalingInstances",
		"autoscaling:DescribeLaunchConfigurations",
	)
}

// AddAWSEBSCSIDriverPermissions appens policy statements that the AWS EBS CSI Driver needs to operate.
func AddAWSEBSCSIDriverPermissions(p *Policy, appendSnapshotPermissions bool) {

	if appendSnapshotPermissions {
		addSnapshotPersmissions(p)
	}

	p.unconditionalAction.Insert(
		"ec2:DescribeAccountAttributes",    // aws.go
		"ec2:DescribeInstances",            // aws.go
		"ec2:DescribeVolumes",              // aws.go
		"ec2:DescribeVolumesModifications", // aws.go
		"ec2:DescribeTags",                 // aws.go
	)
	p.clusterTaggedAction.Insert(
		"ec2:ModifyVolume",            // aws.go
		"ec2:ModifyInstanceAttribute", // aws.go
		"ec2:AttachVolume",            // aws.go
		"ec2:DeleteVolume",            // aws.go
		"ec2:DetachVolume",            // aws.go
	)

	p.Statement = append(p.Statement,
		&Statement{
			Effect: StatementEffectAllow,
			Action: stringorslice.Slice([]string{
				"ec2:CreateVolume", // aws.go
			}),

			Resource: stringorslice.String("*"),
			Condition: Condition{
				"StringEquals": map[string]string{
					"aws:RequestTag/KubernetesCluster": p.clusterName,
				},
			},
		},

		&Statement{
			Effect: StatementEffectAllow,
			Action: stringorslice.String(
				"ec2:CreateTags", // aws.go, tag.go
			),

			Resource: stringorslice.Slice(
				[]string{
					"arn:aws:ec2:*:*:volume/*",
					"arn:aws:ec2:*:*:snapshot/*",
				},
			),
			Condition: Condition{
				"StringEquals": map[string]interface{}{
					"ec2:CreateAction": []string{
						"CreateVolume",
						"CreateSnapshot",
					},
				},
			},
		},

		&Statement{
			Effect: StatementEffectAllow,
			Action: stringorslice.String(
				"ec2:DeleteTags", // aws.go, tag.go
			),
			Resource: stringorslice.Slice(
				[]string{
					"arn:aws:ec2:*:*:volume/*",
					"arn:aws:ec2:*:*:snapshot/*",
				},
			),
			Condition: Condition{
				"StringEquals": map[string]string{
					"aws:ResourceTag/KubernetesCluster": p.clusterName,
				},
			},
		},
	)
}

func addSnapshotPersmissions(p *Policy) {
	p.unconditionalAction.Insert(
		"ec2:CreateSnapshot",
		"ec2:DescribeAvailabilityZones",
		"ec2:DescribeSnapshots",
	)
	p.clusterTaggedAction.Insert(
		"ec2:DeleteSnapshot",
	)

}

// AddDNSControllerPermissions adds IAM permissions used by the dns-controller.
// TODO: Move this to dnscontroller, but it requires moving a lot of code around.
func AddDNSControllerPermissions(b *PolicyBuilder, p *Policy) {
	// Permissions to mutate the specific zone
	if b.HostedZoneID == "" {
		return
	}

	// TODO: Route53 currently not supported in China, need to check and fail/return
	// Remove /hostedzone/ prefix (if present)
	hostedZoneID := strings.TrimPrefix(b.HostedZoneID, "/")
	hostedZoneID = strings.TrimPrefix(hostedZoneID, "hostedzone/")

	p.Statement = append(p.Statement, &Statement{
		Effect: StatementEffectAllow,
		Action: stringorslice.Of("route53:ChangeResourceRecordSets",
			"route53:ListResourceRecordSets",
			"route53:GetHostedZone"),
		Resource: stringorslice.Slice([]string{b.IAMPrefix() + ":route53:::hostedzone/" + hostedZoneID}),
	})

	p.Statement = append(p.Statement, &Statement{
		Effect:   StatementEffectAllow,
		Action:   stringorslice.Slice([]string{"route53:GetChange"}),
		Resource: stringorslice.Slice([]string{b.IAMPrefix() + ":route53:::change/*"}),
	})

	wildcard := stringorslice.Slice([]string{"*"})
	p.Statement = append(p.Statement, &Statement{
		Effect:   StatementEffectAllow,
		Action:   stringorslice.Slice([]string{"route53:ListHostedZones"}),
		Resource: wildcard,
	})
}

func addKMSIAMPolicies(p *Policy, resource stringorslice.StringOrSlice) {
	// TODO could use "kms:ViaService" Condition Key here?
	p.unconditionalAction.Insert(
		"kms:CreateGrant",
		"kms:Decrypt",
		"kms:DescribeKey",
		"kms:Encrypt",
		"kms:GenerateDataKey*",
		"kms:ReEncrypt*",
	)
}

func addKMSGenerateRandomPolicies(p *Policy) {
	// For nodeup to seed the instance's random number generator.
	p.unconditionalAction.Insert(
		"kms:GenerateRandom",
	)
}

func addNodeEC2Policies(p *Policy) {
	// Protokube makes a DescribeInstances call, DescribeRegions when finding S3 State Bucket
	p.unconditionalAction.Insert(
		"ec2:DescribeInstances", "ec2:DescribeRegions",
	)
}

func AddMasterEC2Policies(p *Policy) {
	// Describe* calls don't support any additional IAM restrictions
	// The non-Describe* ec2 calls support different types of filtering:
	// http://docs.aws.amazon.com/AWSEC2/latest/APIReference/ec2-api-permissions.html
	// We try to lock down the permissions here in non-legacy mode,
	// but there are still some improvements we can make:

	// CreateVolume - supports filtering on tags, but we need to switch to pass tags to CreateVolume
	// CreateTags - supports filtering on existing tags. Also supports filtering on VPC for some resources (e.g. security groups)
	// Network Routing Permissions - May not be required with the CNI Networking provider

	// Comments are which cloudprovider code file makes the call
	p.unconditionalAction.Insert(
		"ec2:DescribeAccountAttributes", // aws.go
		"ec2:DescribeInstances",         // aws.go
		"ec2:DescribeInternetGateways",  // aws.go
		"ec2:DescribeRegions",           // s3context.go
		"ec2:DescribeRouteTables",       // aws.go
		"ec2:DescribeSecurityGroups",    // aws.go
		"ec2:DescribeSubnets",           // aws.go
		"ec2:DescribeVolumes",           // aws.go
		"ec2:CreateSecurityGroup",       // aws.go
		"ec2:CreateTags",                // aws.go, tag.go
		"ec2:ModifyInstanceAttribute",   // aws.go
	)
	p.clusterTaggedAction.Insert(
		"ec2:AttachVolume",                  // aws.go
		"ec2:AuthorizeSecurityGroupIngress", // aws.go
		"ec2:CreateRoute",                   // aws.go
		"ec2:DeleteRoute",                   // aws.go
		"ec2:DeleteSecurityGroup",           // aws.go
		"ec2:RevokeSecurityGroupIngress",    // aws.go
	)
}

func AddMasterELBPolicies(p *Policy) {
	// Comments are which cloudprovider code file makes the call
	p.unconditionalAction.Insert(
		"ec2:DescribeVpcs",                                             // aws_loadbalancer.go
		"elasticloadbalancing:DescribeLoadBalancers",                   // aws.go
		"elasticloadbalancing:DescribeLoadBalancerAttributes",          // aws.go
		"elasticloadbalancing:DescribeListeners",                       // aws_loadbalancer.go
		"elasticloadbalancing:DescribeLoadBalancerPolicies",            // aws_loadbalancer.go
		"elasticloadbalancing:DescribeTargetGroups",                    // aws_loadbalancer.go
		"elasticloadbalancing:DescribeTargetHealth",                    // aws_loadbalancer.go
		"elasticloadbalancing:CreateListener",                          // aws_loadbalancer.go
		"elasticloadbalancing:CreateTargetGroup",                       // aws_loadbalancer.go
		"elasticloadbalancing:CreateLoadBalancer",                      // aws_loadbalancer.go
		"elasticloadbalancing:CreateLoadBalancerPolicy",                // aws_loadbalancer.go
		"elasticloadbalancing:CreateLoadBalancerListeners",             // aws_loadbalancer.go
		"elasticloadbalancing:DeleteLoadBalancer",                      // aws.go
		"elasticloadbalancing:DeleteLoadBalancerListeners",             // aws_loadbalancer.go
		"elasticloadbalancing:DeleteListener",                          // aws_loadbalancer.go
		"elasticloadbalancing:DeleteTargetGroup",                       // aws_loadbalancer.go
		"elasticloadbalancing:AddTags",                                 // aws_loadbalancer.go
		"elasticloadbalancing:ModifyLoadBalancerAttributes",            // aws_loadbalancer.go
		"elasticloadbalancing:ModifyListener",                          // aws_loadbalancer.go
		"elasticloadbalancing:ModifyTargetGroup",                       // aws_loadbalancer.go
		"elasticloadbalancing:AttachLoadBalancerToSubnets",             // aws_loadbalancer.go
		"elasticloadbalancing:ApplySecurityGroupsToLoadBalancer",       // aws_loadbalancer.go
		"elasticloadbalancing:ConfigureHealthCheck",                    // aws_loadbalancer.go
		"elasticloadbalancing:DetachLoadBalancerFromSubnets",           // aws_loadbalancer.go
		"elasticloadbalancing:DeregisterInstancesFromLoadBalancer",     // aws_loadbalancer.go
		"elasticloadbalancing:RegisterInstancesWithLoadBalancer",       // aws_loadbalancer.go
		"elasticloadbalancing:SetLoadBalancerPoliciesForBackendServer", // aws_loadbalancer.go
		"elasticloadbalancing:DeregisterTargets",                       // aws_loadbalancer.go
		"elasticloadbalancing:RegisterTargets",                         // aws_loadbalancer.go
		"elasticloadbalancing:SetLoadBalancerPoliciesOfListener",       // aws_loadbalancer.go
	)
}

func addMasterASPolicies(p *Policy) {
	// Comments are which cloudprovider / autoscaler code file makes the call
	// TODO: Make optional only if using autoscalers
	p.unconditionalAction.Insert(
		"autoscaling:DescribeAutoScalingGroups",    // aws_instancegroups.go
		"autoscaling:DescribeLaunchConfigurations", // aws.go
		"autoscaling:DescribeTags",                 // auto_scaling.go
		"ec2:DescribeLaunchTemplateVersions",
	)
	p.clusterTaggedAction.Insert(
		"autoscaling:CompleteLifecycleAction",      // aws_manager.go
		"autoscaling:DescribeAutoScalingInstances", // aws_instancegroups.go
	)
}

func addASLifecyclePolicies(p *Policy, enableHookSupport bool) {
	if enableHookSupport {
		p.clusterTaggedAction.Insert(
			"autoscaling:CompleteLifecycleAction", // aws_manager.go
		)
		p.unconditionalAction.Insert(
			"autoscaling:DescribeLifecycleHooks",
		)
	}
	p.unconditionalAction.Insert(
		"autoscaling:DescribeAutoScalingInstances",
	)
}

func addCertIAMPolicies(p *Policy) {
	// TODO: Make optional only if using IAM SSL Certs on ELBs
	p.unconditionalAction.Insert(
		"iam:ListServerCertificates",
		"iam:GetServerCertificate",
	)
}

func addLyftVPCPermissions(p *Policy) {
	p.unconditionalAction.Insert(
		"ec2:AssignPrivateIpAddresses",
		"ec2:AttachNetworkInterface",
		"ec2:CreateNetworkInterface",
		"ec2:DeleteNetworkInterface",
		"ec2:DescribeInstanceTypes",
		"ec2:DescribeNetworkInterfaces",
		"ec2:DescribeSecurityGroups",
		"ec2:DescribeSubnets",
		"ec2:DescribeVpcPeeringConnections",
		"ec2:DescribeVpcs",
		"ec2:DetachNetworkInterface",
		"ec2:ModifyNetworkInterfaceAttribute",
		"ec2:UnassignPrivateIpAddresses",
	)
}

func addCiliumEniPermissions(p *Policy) {
	p.unconditionalAction.Insert(
		"ec2:DescribeSubnets",
		"ec2:AttachNetworkInterface",
		"ec2:AssignPrivateIpAddresses",
		"ec2:UnassignPrivateIpAddresses",
		"ec2:CreateNetworkInterface",
		"ec2:DescribeNetworkInterfaces",
		"ec2:DescribeVpcPeeringConnections",
		"ec2:DescribeSecurityGroups",
		"ec2:DetachNetworkInterface",
		"ec2:DeleteNetworkInterface",
		"ec2:ModifyNetworkInterfaceAttribute",
		"ec2:DescribeVpcs",
	)
}

func addAmazonVPCCNIPermissions(p *Policy, iamPrefix string) {
	p.unconditionalAction.Insert(
		"ec2:AssignPrivateIpAddresses",
		"ec2:AttachNetworkInterface",
		"ec2:CreateNetworkInterface",
		"ec2:DeleteNetworkInterface",
		"ec2:DescribeInstances",
		"ec2:DescribeInstanceTypes",
		"ec2:DescribeTags",
		"ec2:DescribeNetworkInterfaces",
		"ec2:DetachNetworkInterface",
		"ec2:ModifyNetworkInterfaceAttribute",
		"ec2:UnassignPrivateIpAddresses",
	)
	p.Statement = append(p.Statement,
		&Statement{
			Effect: StatementEffectAllow,
			Action: stringorslice.Slice([]string{
				"ec2:CreateTags",
			}),
			Resource: stringorslice.Slice([]string{
				strings.Join([]string{iamPrefix, ":ec2:*:*:network-interface/*"}, ""),
			})},
	)
}

func addNodeTerminationHandlerSQSPermissions(p *Policy) {
	p.unconditionalAction.Insert(
		"autoscaling:CompleteLifecycleAction",
		"autoscaling:DescribeAutoScalingInstances",
		"sqs:DeleteMessage",
		"sqs:ReceiveMessage",
	)
}
