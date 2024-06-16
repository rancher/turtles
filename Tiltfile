# -*- mode: Python -*-

# Originally based on the Tiltfile from the Cluster API project

load("./tilt/project/Tiltfile", "project_enable")
load("./tilt/io/Tiltfile", "info", "warn", "file_write")
load('ext://namespace', 'namespace_create')


# set defaults
version_settings(True, ">=0.22.2")

settings = {
    "k8s_context": os.getenv("RT_K8S_CONTEXT", "rancher-desktop"),
    "debug": {},
    "default_registry": "docker.io/rancher"
}

# global settings
tilt_file = "./tilt-settings.yaml" if os.path.exists("./tilt-settings.yaml") else "./tilt-settings.json"
settings.update(read_yaml(
    tilt_file,
    default = {},
))

k8s_ctx = settings.get("k8s_context")
allow_k8s_contexts(k8s_ctx)

os_name = str(local("go env GOOS")).rstrip("\n")
os_arch = str(local("go env GOARCH")).rstrip("\n")

if settings.get("trigger_mode") == "manual":
    trigger_mode(TRIGGER_MODE_MANUAL)

if settings.get("default_registry") != "":
    default_registry(settings.get("default_registry"))

always_enable_projects = ["turtles"]

projects = {
    "turtles": {
        "context": ".",
        "image": "ghcr.io/rancher/turtles:dev",
        "live_reload_deps": [
            "main.go",
            "go.mod",
            "go.sum",
            "internal",
            "features",
        ],
        "kustomize_dir": "config/default",
        "label": "turtles"
    }
}

# Users may define their own Tilt customizations in tilt.d. This directory is excluded from git and these files will
# not be checked in to version control.
def include_user_tilt_files():
    user_tiltfiles = listdir("tilt.d")
    for f in user_tiltfiles:
        include(f)

def enable_projects():
    for name in get_projects():
        p = projects.get(name)
        info(p)
        project_enable(name, p, settings.get("debug").get(name, {}))

def get_projects():
    user_enable_projects = settings.get("enable_projects", [])
    return {k: "" for k in user_enable_projects + always_enable_projects}.keys()


##############################
# Actual work happens here
##############################

include_user_tilt_files()

enable_projects()