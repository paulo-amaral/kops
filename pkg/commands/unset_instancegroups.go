/*
Copyright 2021 The Kubernetes Authors.

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

package commands

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"k8s.io/kops/cmd/kops/util"
	api "k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/featureflag"
	"k8s.io/kops/util/pkg/reflectutils"
)

// UnsetInstanceGroupOptions contains the options for unsetting configuration on an
// instance group.
type UnsetInstanceGroupOptions struct {
	Fields            []string
	ClusterName       string
	InstanceGroupName string
}

// RunUnsetInstancegroup implements the unset instancegroup command logic.
func RunUnsetInstancegroup(ctx context.Context, f *util.Factory, cmd *cobra.Command, out io.Writer, options *UnsetInstanceGroupOptions) error {
	if !featureflag.SpecOverrideFlag.Enabled() {
		return fmt.Errorf("unset instancegroup is currently feature gated; set `export KOPS_FEATURE_FLAGS=SpecOverrideFlag`")
	}

	if options.ClusterName == "" {
		return field.Required(field.NewPath("clusterName"), "Cluster name is required")
	}
	if options.InstanceGroupName == "" {
		return field.Required(field.NewPath("instancegroupName"), "Instance Group name is required")
	}

	clientset, err := f.Clientset()
	if err != nil {
		return err
	}

	cluster, err := clientset.GetCluster(ctx, options.ClusterName)
	if err != nil {
		return err
	}

	instanceGroups, err := ReadAllInstanceGroups(ctx, clientset, cluster)
	if err != nil {
		return err
	}
	var instanceGroupToUpdate *api.InstanceGroup
	for _, instanceGroup := range instanceGroups {
		if instanceGroup.GetName() == options.InstanceGroupName {
			instanceGroupToUpdate = instanceGroup
		}
	}
	if instanceGroupToUpdate == nil {
		return fmt.Errorf("unable to find instance group with name %q", options.InstanceGroupName)
	}

	err = UnsetInstancegroupFields(options.Fields, instanceGroupToUpdate)
	if err != nil {
		return err
	}

	err = UpdateInstanceGroup(ctx, clientset, cluster, instanceGroups, instanceGroupToUpdate)
	if err != nil {
		return err
	}

	return nil
}

// UnsetInstancegroupFields sets field values in the instance group.
func UnsetInstancegroupFields(fields []string, instanceGroup *api.InstanceGroup) error {
	for _, field := range fields {
		key := strings.TrimPrefix(field, "instancegroup.")

		if err := reflectutils.Unset(instanceGroup, key); err != nil {
			return err
		}
	}

	return nil
}
