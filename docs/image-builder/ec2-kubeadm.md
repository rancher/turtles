# Building Ubuntu 24.04 AMIs with Kubernetes for CAPI

This guide walks you through building AWS AMIs for Cluster API with Ubuntu 24.04 and any Kubernetes version you need.

## Getting Started

### 1. Clone the Repository

First, clone the image-builder repo:

```bash
git clone git@github.com:kubernetes-sigs/image-builder.git
cd image-builder/images/capi
```

### 2. AWS Prerequisites

You'll need:
- An AWS account with EC2 permissions to create AMIs.
- AWS CLI installed and configured

## Building AMI

### Step 1: Install Dependencies

The build process needs Packer and Ansible. Install them with:

```bash
make deps-ami
```

This installs Python, Ansible, Packer, and initializes Packer plugins. If you're on macOS, the tools get installed to `.local/bin` in the current directory. Add them to your PATH:

```bash
export PATH=$PWD/.local/bin:$PATH
```

### Step 2: Choose Your Kubernetes Version

Create a config file with the Kubernetes version you want:

**For Kubernetes v1.33.5:**
```bash
cat > my-k8s-config.json <<EOF
{
  "aws_region": "eu-west-2",
  "ami_regions": "",
  "ami_groups": "",
  "snapshot_groups": "",
  "kubernetes_deb_version": "1.33.5-1.1",
  "kubernetes_rpm_version": "1.33.5",
  "kubernetes_semver": "v1.33.5",
  "kubernetes_series": "v1.33"
}
EOF
```

### Step 3: Build the AMI

Build Ubuntu 24.04 with your chosen Kubernetes version:

```bash
PACKER_VAR_FILES="$(pwd)/my-k8s-config.json" make build-ami-ubuntu-2404
```

What happens:
1. Packer launches a temporary EC2 instance
2. Installs Kubernetes and dependencies
3. Runs Ansible playbooks to configure everything
4. Creates an AMI snapshot
5. Cleans up the temporary instance

Build time is usually 10-20 minutes.

### Step 4: Save Your AMI ID

When the build finishes, you'll see:

```bash
==> Builds finished. The artifacts of successful builds are:
--> amazon-ebs.ubuntu-2404: AMIs were created:
eu-west-2: ami-0abc123def456789
```

Save that AMI ID - you'll need it for your CAPI clusters.
