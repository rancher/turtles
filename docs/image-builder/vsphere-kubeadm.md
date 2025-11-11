# Building Ubuntu 24.04 vSphere OVAs with Kubernetes for CAPI

This guide walks you through building vSphere OVA templates for Cluster API with Ubuntu 24.04 and any Kubernetes version you need.

## Getting Started

### 1. Clone the Repository

First, clone the image-builder repo:

```bash
git clone git@github.com:kubernetes-sigs/image-builder.git
cd image-builder/images/capi
```

### 2. vSphere Prerequisites

You'll need:
- A vSphere environment
- vSphere credentials with permissions to create VMs and templates

### 3. Create vSphere Credentials File

Create a `vsphere.json` file with your vSphere credentials:

```bash
cat > packer/ova/vsphere.json <<EOF
{
  "vcenter_server": "vcenter.example.com",
  "username": "administrator@vsphere.local",
  "password": "your-password",
  "datacenter": "Datacenter",
  "cluster": "Cluster",
  "datastore": "datastore1",
  "folder": "Templates",
  "network": "VM Network",
  "insecure_connection": "true"
}
EOF
```

## Building vSphere OVA

### Step 1: Install Dependencies

The build process needs Packer and Ansible. Install them with:

```bash
make deps-ova
```

This installs Python, Ansible, Packer, and initializes Packer plugins. If you're on macOS, the tools get installed to `.local/bin` in the current directory. Add them to your PATH:

```bash
export PATH="$(pwd)/.local/bin:$PATH"
```

### Step 2: Choose Your Kubernetes Version

Create a config file with the Kubernetes version you want:

**For Kubernetes v1.33.5:**
```bash
cat > my-k8s-config.json <<EOF
{
  "kubernetes_deb_version": "1.33.5-1.1",
  "kubernetes_rpm_version": "1.33.5",
  "kubernetes_semver": "v1.33.5",
  "kubernetes_series": "v1.33"
}
EOF
```

### Step 3: Build the vSphere OVA

Build Ubuntu 24.04 with your chosen Kubernetes version:

```bash
PACKER_VAR_FILES="$(pwd)/my-k8s-config.json" make build-node-ova-vsphere-ubuntu-2404
```

What happens:
1. Packer connects to vSphere
2. Creates a new VM from Ubuntu 24.04 ISO
3. Installs Kubernetes and dependencies
4. Runs Ansible playbooks to configure everything
5. Converts the VM to a template
6. Cleans up the temporary build VM

Build time is usually 1-1.5 hours due to image download/upload.

### Step 4: Verify Your Template

When the build finishes, check that your template was created in vSphere:

1. Log in to vCenter
2. Navigate to your configured folder
3. Look for a template named like: `ubuntu-2404-kube-v1.33.5`

Save the template name - you'll need it for your CAPI clusters.
