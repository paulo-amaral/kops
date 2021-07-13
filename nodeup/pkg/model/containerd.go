/*
Copyright 2019 The Kubernetes Authors.

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

package model

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blang/semver/v4"
	"k8s.io/klog/v2"
	"k8s.io/kops/nodeup/pkg/model/resources"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/flagbuilder"
	"k8s.io/kops/pkg/model/components"
	"k8s.io/kops/pkg/systemd"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/nodeup/nodetasks"
	"k8s.io/kops/util/pkg/distributions"
)

// ContainerdBuilder install containerd (just the packages at the moment)
type ContainerdBuilder struct {
	*NodeupModelContext
}

var _ fi.ModelBuilder = &ContainerdBuilder{}

// Build is responsible for configuring the containerd daemon
func (b *ContainerdBuilder) Build(c *fi.ModelBuilderContext) error {
	if b.skipInstall() {
		klog.Infof("SkipInstall is set to true; won't install containerd")
		return nil
	}

	installContainerd := true

	// @check: neither flatcar nor containeros need provision containerd.service, just the containerd daemon options
	switch b.Distribution {
	case distributions.DistributionFlatcar:
		klog.Infof("Detected Flatcar; won't install containerd")
		installContainerd = false
		if b.Cluster.Spec.ContainerRuntime == "containerd" {
			b.buildSystemdServiceOverrideFlatcar(c)
		}
	case distributions.DistributionContainerOS:
		klog.Infof("Detected ContainerOS; won't install containerd")
		installContainerd = false
		b.buildSystemdServiceOverrideContainerOS(c)
	}

	if b.Cluster.Spec.ContainerRuntime == "containerd" {
		// Using containerd with Kubenet requires special configuration.
		// This is a temporary backwards-compatible solution for kubenet users and will be deprecated when Kubenet is deprecated:
		// https://github.com/containerd/containerd/blob/master/docs/cri/config.md#cni-config-template
		if components.UsesKubenet(b.Cluster.Spec.Networking) {
			b.buildCNIConfigTemplateFile(c)
			if err := b.buildIPMasqueradeRules(c); err != nil {
				return err
			}
		}
	}

	// If there are containerd configuration overrides, apply them
	b.buildConfigFile(c)

	if installContainerd {
		if err := b.installContainerd(c); err != nil {
			return err
		}
	}

	return nil
}

// installContainerd installs the binaries and services to run containerd.
// We break it out because on immutable OSes we only configure containerd, we don't install it.
func (b *ContainerdBuilder) installContainerd(c *fi.ModelBuilderContext) error {
	// Add Apache2 license
	{
		t := &nodetasks.File{
			Path:     "/usr/share/doc/containerd/apache.txt",
			Contents: fi.NewStringResource(resources.ContainerdApache2License),
			Type:     nodetasks.FileType_File,
		}
		c.AddTask(t)
	}

	// Add binaries from assets
	if b.Cluster.Spec.ContainerRuntime == "containerd" {
		f := b.Assets.FindMatches(regexp.MustCompile(`^(\./)?usr/local/(bin/containerd|bin/crictl|bin/ctr|sbin/runc)`))
		if len(f) == 0 {
			f = b.Assets.FindMatches(regexp.MustCompile(`^docker/(containerd|ctr|runc)`))
		}
		if len(f) == 0 {
			return fmt.Errorf("unable to find any containerd binaries in assets")
		}
		for k, v := range f {
			fileTask := &nodetasks.File{
				Path:     filepath.Join("/usr/bin", k),
				Contents: v,
				Type:     nodetasks.FileType_File,
				Mode:     fi.String("0755"),
			}
			c.AddTask(fileTask)
		}

		// Add configuration file for easier use of crictl
		b.addCrictlConfig(c)
	}

	var containerRuntimeVersion string
	if b.Cluster.Spec.ContainerRuntime == "containerd" {
		if b.Cluster.Spec.Containerd != nil {
			containerRuntimeVersion = fi.StringValue(b.Cluster.Spec.Containerd.Version)
		} else {
			return fmt.Errorf("error finding contained version")
		}
	} else {
		if b.Cluster.Spec.Docker != nil {
			containerRuntimeVersion = fi.StringValue(b.Cluster.Spec.Docker.Version)
		} else {
			return fmt.Errorf("error finding Docker version")
		}
	}
	sv, err := semver.ParseTolerant(containerRuntimeVersion)
	if err != nil {
		return fmt.Errorf("error parsing container runtime version %q: %v", containerRuntimeVersion, err)
	}
	c.AddTask(b.buildSystemdService(sv))

	if err := b.buildSysconfigFile(c); err != nil {
		return err
	}

	return nil
}

func (b *ContainerdBuilder) buildSystemdService(sv semver.Version) *nodetasks.Service {
	// Based on https://github.com/containerd/containerd/blob/master/containerd.service

	manifest := &systemd.Manifest{}
	manifest.Set("Unit", "Description", "containerd container runtime")
	manifest.Set("Unit", "Documentation", "https://containerd.io")
	manifest.Set("Unit", "After", "network.target local-fs.target")

	// Restore the default SELinux security contexts for the containerd and runc binaries
	if b.Distribution.IsRHELFamily() && b.Cluster.Spec.Docker != nil && fi.BoolValue(b.Cluster.Spec.Docker.SelinuxEnabled) {
		manifest.Set("Service", "ExecStartPre", "/bin/sh -c 'restorecon -v /usr/bin/runc'")
		manifest.Set("Service", "ExecStartPre", "/bin/sh -c 'restorecon -v /usr/bin/containerd*'")
	}

	manifest.Set("Service", "EnvironmentFile", "/etc/sysconfig/containerd")
	manifest.Set("Service", "EnvironmentFile", "/etc/environment")
	manifest.Set("Service", "ExecStartPre", "-/sbin/modprobe overlay")
	manifest.Set("Service", "ExecStart", "/usr/bin/containerd -c "+b.containerdConfigFilePath()+" \"$CONTAINERD_OPTS\"")

	// notify the daemon's readiness to systemd
	if (b.Cluster.Spec.ContainerRuntime == "containerd" && sv.GTE(semver.MustParse("1.3.4"))) || sv.GTE(semver.MustParse("19.3.13")) {
		manifest.Set("Service", "Type", "notify")
	}

	// set delegate yes so that systemd does not reset the cgroups of containerd containers
	manifest.Set("Service", "Delegate", "yes")
	// kill only the containerd process, not all processes in the cgroup
	manifest.Set("Service", "KillMode", "process")

	manifest.Set("Service", "Restart", "always")
	manifest.Set("Service", "RestartSec", "5")

	manifest.Set("Service", "LimitNPROC", "infinity")
	manifest.Set("Service", "LimitCORE", "infinity")
	manifest.Set("Service", "LimitNOFILE", "infinity")
	manifest.Set("Service", "TasksMax", "infinity")

	// make killing of processes of this unit under memory pressure very unlikely
	manifest.Set("Service", "OOMScoreAdjust", "-999")

	manifest.Set("Install", "WantedBy", "multi-user.target")

	manifestString := manifest.Render()
	klog.V(8).Infof("Built service manifest %q\n%s", "containerd", manifestString)

	service := &nodetasks.Service{
		Name:       "containerd.service",
		Definition: s(manifestString),
	}

	service.InitDefaults()

	return service
}

// containerdConfigFilePath returns the path we use for the containerd config file
// We normally use a different path for clarity, but on some OSes we can't override the path.
// TODO: Should we just use config.toml everywhere?
func (b *ContainerdBuilder) containerdConfigFilePath() string {
	switch b.Distribution {
	case distributions.DistributionContainerOS:
		return "/etc/containerd/config.toml"
	default:
		return "/etc/containerd/config-kops.toml"
	}
}

// buildSystemdServiceOverrideContainerOS is responsible for overriding the containerd service for ContainerOS
func (b *ContainerdBuilder) buildSystemdServiceOverrideContainerOS(c *fi.ModelBuilderContext) {
	lines := []string{
		"[Service]",
		"EnvironmentFile=/etc/environment",
		"TasksMax=infinity",
	}
	contents := strings.Join(lines, "\n")

	c.AddTask(&nodetasks.File{
		Path:       "/etc/systemd/system/containerd.service.d/10-kops.conf",
		Contents:   fi.NewStringResource(contents),
		Type:       nodetasks.FileType_File,
		AfterFiles: []string{b.containerdConfigFilePath()},
		OnChangeExecute: [][]string{
			{"systemctl", "daemon-reload"},
			{"systemctl", "restart", "containerd.service"},
			// We need to restart kops-configuration service since nodeup needs to load images
			// into containerd with the new config. We restart in the background because
			// kops-configuration is of type "one-shot", so the restart command will wait for
			// nodeup to finish executing.
			{"systemctl", "restart", "kops-configuration.service", "&"},
		},
	})
}

// buildSystemdServiceOverrideFlatcar is responsible for overriding the containerd service for Flatcar
func (b *ContainerdBuilder) buildSystemdServiceOverrideFlatcar(c *fi.ModelBuilderContext) {
	lines := []string{
		"[Service]",
		"Environment=CONTAINERD_CONFIG=" + b.containerdConfigFilePath(),
		"EnvironmentFile=/etc/environment",
	}
	contents := strings.Join(lines, "\n")

	c.AddTask(&nodetasks.File{
		Path:       "/etc/systemd/system/containerd.service.d/10-kops.conf",
		Contents:   fi.NewStringResource(contents),
		Type:       nodetasks.FileType_File,
		AfterFiles: []string{b.containerdConfigFilePath()},
		OnChangeExecute: [][]string{
			{"systemctl", "daemon-reload"},
			{"systemctl", "restart", "containerd.service"},
			// We need to restart kops-configuration service since nodeup needs to load images
			// into containerd with the new config. We restart in the background because
			// kops-configuration is of type "one-shot", so the restart command will wait for
			// nodeup to finish executing.
			{"systemctl", "restart", "kops-configuration.service", "&"},
		},
	})
}

// buildSysconfigFile is responsible for creating the containerd sysconfig file
func (b *ContainerdBuilder) buildSysconfigFile(c *fi.ModelBuilderContext) error {
	var containerd kops.ContainerdConfig
	if b.Cluster.Spec.Containerd != nil {
		containerd = *b.Cluster.Spec.Containerd
	}

	flagsString, err := flagbuilder.BuildFlags(&containerd)
	if err != nil {
		return fmt.Errorf("error building containerd flags: %v", err)
	}

	lines := []string{
		"CONTAINERD_OPTS=" + flagsString,
	}
	contents := strings.Join(lines, "\n")

	c.AddTask(&nodetasks.File{
		Path:     "/etc/sysconfig/containerd",
		Contents: fi.NewStringResource(contents),
		Type:     nodetasks.FileType_File,
	})

	return nil
}

// buildConfigFile is responsible for creating the containerd configuration file
func (b *ContainerdBuilder) buildConfigFile(c *fi.ModelBuilderContext) {
	c.AddTask(&nodetasks.File{
		Path:     b.containerdConfigFilePath(),
		Contents: fi.NewStringResource(b.NodeupConfig.ContainerdConfig),
		Type:     nodetasks.FileType_File,
	})
}

// skipInstall determines if kops should skip the installation and configuration of containerd
func (b *ContainerdBuilder) skipInstall() bool {
	d := b.Cluster.Spec.Containerd

	// don't skip install if the user hasn't specified anything
	if d == nil {
		return false
	}

	return d.SkipInstall
}

// addCritctlConfig creates /etc/crictl.yaml, which lets crictl work out-of-the-box.
func (b *ContainerdBuilder) addCrictlConfig(c *fi.ModelBuilderContext) {
	conf := `
runtime-endpoint: unix:///run/containerd/containerd.sock
`

	c.AddTask(&nodetasks.File{
		Path:     "/etc/crictl.yaml",
		Contents: fi.NewStringResource(conf),
		Type:     nodetasks.FileType_File,
	})
}

// buildIPMasqueradeRules creates the DNAT rules.
// Network modes where pods don't have "real network" IPs, use NAT so that they assume the IP of the node.
func (b *ContainerdBuilder) buildIPMasqueradeRules(c *fi.ModelBuilderContext) error {
	// TODO: Should we just rely on running nodeup on every boot, instead of setting up a systemd unit?

	// This is based on rules from gce/cos/configure-helper.sh and the old logic in kubenet_linux.go

	// We stick closer to the logic in kubenet_linux, both for compatibility, and because the GCE logic
	// skips masquerading for all private CIDR ranges, but this depends on an assumption that is likely GCE-specific.
	// On GCE custom routes are at the network level, on AWS they are at the route-table / subnet level.
	// We cannot generally assume that because something is in the private network space, that it can reach us.
	// If we adopt "native" pod IPs (GCE ip-alias, AWS VPC CNI, etc) we can likely move to rules closer to the upstream ones.
	script := `#!/bin/bash
# Built by kOps - do not edit

iptables -w -t nat -N IP-MASQ
iptables -w -t nat -A POSTROUTING -m comment --comment "ip-masq: ensure nat POSTROUTING directs all non-LOCAL destination traffic to our custom IP-MASQ chain" -m addrtype ! --dst-type LOCAL -j IP-MASQ
iptables -w -t nat -A IP-MASQ -d {{.NonMasqueradeCIDR}} -m comment --comment "ip-masq: pod cidr is not subject to MASQUERADE" -j RETURN
iptables -w -t nat -A IP-MASQ -m comment --comment "ip-masq: outbound traffic is subject to MASQUERADE (must be last in chain)" -j MASQUERADE
`

	if b.Cluster.Spec.NonMasqueradeCIDR == "" {
		// We could fall back to the pod CIDR, that is likely more correct anyway
		return fmt.Errorf("NonMasqueradeCIDR is not set")
	}

	script = strings.ReplaceAll(script, "{{.NonMasqueradeCIDR}}", b.Cluster.Spec.NonMasqueradeCIDR)

	c.AddTask(&nodetasks.File{
		Path:     "/opt/kops/bin/cni-iptables-setup",
		Contents: fi.NewStringResource(script),
		Type:     nodetasks.FileType_File,
		Mode:     s("0755"),
	})

	manifest := &systemd.Manifest{}
	manifest.Set("Unit", "Description", "Configure iptables for kubernetes CNI")
	manifest.Set("Unit", "Documentation", "https://github.com/kubernetes/kops")
	manifest.Set("Unit", "Before", "network.target")
	manifest.Set("Service", "Type", "oneshot")
	manifest.Set("Service", "RemainAfterExit", "yes")
	manifest.Set("Service", "ExecStart", "/opt/kops/bin/cni-iptables-setup")
	manifest.Set("Install", "WantedBy", "basic.target")

	manifestString := manifest.Render()
	klog.V(8).Infof("Built service manifest %q\n%s", "cni-iptables-setup", manifestString)

	service := &nodetasks.Service{
		Name:       "cni-iptables-setup.service",
		Definition: s(manifestString),
	}
	service.InitDefaults()
	c.AddTask(service)

	return nil
}

// buildCNIConfigTemplateFile is responsible for creating a special template for setups using Kubenet
func (b *ContainerdBuilder) buildCNIConfigTemplateFile(c *fi.ModelBuilderContext) {

	// Based on https://github.com/kubernetes/kubernetes/blob/15a8a8ec4a3275a33b7f8eb3d4d98db2abad55b7/cluster/gce/gci/configure-helper.sh#L2911-L2937

	contents := `{
    "cniVersion": "0.4.0",
    "name": "k8s-pod-network",
    "plugins": [
        {
            "type": "ptp",
            "ipam": {
                "type": "host-local",
                "ranges": [[{"subnet": "{{.PodCIDR}}"}]],
                "routes": [{ "dst": "0.0.0.0/0" }]
            }
        },
        {
            "type": "portmap",
            "capabilities": {"portMappings": true}
        }
    ]
}
`
	klog.V(8).Infof("Built containerd CNI config template\n%s", contents)

	c.AddTask(&nodetasks.File{
		Path:     "/etc/containerd/config-cni.template",
		Contents: fi.NewStringResource(contents),
		Type:     nodetasks.FileType_File,
	})
}
