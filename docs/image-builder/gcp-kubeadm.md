# Building Ubuntu 24.04 GCP Images with Kubernetes for CAPI

This guide walks you through building GCP images for Cluster API with Ubuntu 24.04 and any Kubernetes version you need.

## Getting Started

### 1. Clone the Repository

First, clone the image-builder repo:

```bash
git clone git@github.com:kubernetes-sigs/image-builder.git
cd image-builder/images/capi
```

### 2. GCP Prerequisites

You'll need:
- `gcloud` CLI installed and configured
- A service account with appropriate permissions

#### Set Up Service Account

If needed, create a service account with the necessary permissions. We'll assume there is already one called `capg-packer-service-account` with the required roles.

Create a key for your service account:

```bash
export GCP_PROJECT_ID="your-project-id"
export SERVICE_ACCOUNT_NAME="capg-packer-service-account"

gcloud iam service-accounts keys create capg-packer-key.json \
  --iam-account=${SERVICE_ACCOUNT_NAME}@${GCP_PROJECT_ID}.iam.gserviceaccount.com
```

### 3. Set Environment Variables

```bash
export GCP_PROJECT_ID="your-project-id"
export GOOGLE_APPLICATION_CREDENTIALS="$(pwd)/capg-packer-key.json"
```

### 4. Create Firewall Rule for Packer SSH Access

Packer needs SSH access to the temporary build instance. Create a firewall rule:

```bash
gcloud compute firewall-rules create allow-packer-ssh \
  --project=${GCP_PROJECT_ID} \
  --network=default \
  --allow=tcp:22 \
  --source-ranges=0.0.0.0/0 \
  --target-tags=packer
```

This rule only applies to instances tagged with `packer`, which we'll add to the build instance later.

## Building GCP Image

### Step 1: Install Dependencies

The build process needs Packer and Ansible. Install them with:

```bash
make deps-gce
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
  "zone": "europe-west2-a",
  "kubernetes_deb_version": "1.33.5-1.1",
  "kubernetes_rpm_version": "1.33.5",
  "kubernetes_semver": "v1.33.5",
  "kubernetes_series": "v1.33"
}
EOF
```

### Step 3: Build the GCP Image

Build Ubuntu 24.04 with your chosen Kubernetes version:

```bash
PACKER_VAR_FILES="$(pwd)/my-k8s-config.json" make build-gce-ubuntu-2404
```

**Important:** If the build gets stuck at "Waiting for SSH to become available...", you need to add the `packer` tag to the build instance:

1. Find the instance name:
```bash
gcloud compute instances list \
  --project=${GCP_PROJECT_ID} \
  --filter="name:packer* AND zone:europe-west2-a"
```

2. Add the packer tag:
```bash
gcloud compute instances add-tags INSTANCE_NAME \
  --project=${GCP_PROJECT_ID} \
  --zone=europe-west2-a \
  --tags=packer
```

Replace `INSTANCE_NAME` with the actual instance name from step 1.

What happens during the build:
1. Packer launches a temporary GCP instance
2. Installs Kubernetes and dependencies
3. Runs Ansible playbooks to configure everything
4. Creates a GCP image snapshot
5. Cleans up the temporary instance

Build time is usually 10-20 minutes.

### Step 4: Verify Your Image

When the build finishes, check that your image was created:

```bash
gcloud compute images list \
  --project=${GCP_PROJECT_ID} \
  --no-standard-images \
  --filter="family:capi-ubuntu-2404-k8s-v1-33"
```

You'll see output like:

```
NAME                                          PROJECT         FAMILY                         CREATION_TIMESTAMP
cluster-api-ubuntu-2404-v1.33.5-1234567890    your-project    capi-ubuntu-2404-k8s-v1-33    2024-11-04T10:00:00.000-00:00
```

Save the image name - you'll need it for your CAPI clusters.

### Step 5: Clean Up Firewall Rule

After the build completes, remove the temporary firewall rule:

```bash
gcloud compute firewall-rules delete allow-packer-ssh \
  --project=${GCP_PROJECT_ID} \
  --quiet
```
