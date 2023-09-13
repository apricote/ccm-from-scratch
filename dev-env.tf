terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "1.42.1"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "4.0.4"
    }
    local = {
      source  = "hashicorp/local"
      version = "2.4.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "2.11.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "2.23.0"
    }
  }
}

# Configure the Hetzner Cloud Provider
provider "hcloud" {
  token = var.hcloud_token
}

variable "hcloud_token" {
  type      = string
  sensitive = true
}

variable "worker_count" {
  type    = number
  default = 1
}

locals {
  kubeconfig_path = "${path.module}/kubeconfig.yaml"
  cluster_cidr = "10.244.0.0/16"
}

resource "tls_private_key" "ssh" {
  algorithm = "ED25519"
}

resource "local_sensitive_file" "ssh" {
  content  = tls_private_key.ssh.private_key_openssh
  filename = "${path.module}/.dev-env.ssh"
}

# Create a new SSH key
resource "hcloud_ssh_key" "default" {
  name       = "ccm-from-scratch dev env"
  public_key = tls_private_key.ssh.public_key_openssh
}

resource "hcloud_network" "cluster" {
  name = "ccm-from-scratch"
  ip_range = "10.0.0.0/8"
}

resource "hcloud_network_subnet" "cluster" {
  ip_range     = "10.0.0.0/24"
  network_id   = hcloud_network.cluster.id
  network_zone = "eu-central"
  type         = "cloud"
}

resource "hcloud_server" "cp" {
  name        = "ccm-from-scratch-control"
  server_type = "cpx11"
  location    = "fsn1"
  image       = "ubuntu-22.04"
  ssh_keys    = [hcloud_ssh_key.default.id]

  network {
    network_id = hcloud_network.cluster.id
    alias_ips = []
  }
  depends_on = [hcloud_network_subnet.cluster]

  connection {
    host        = self.ipv4_address
    user        = "root"
    type        = "ssh"
    private_key = tls_private_key.ssh.private_key_openssh
  }

  provisioner "remote-exec" {
    inline = ["cloud-init status --wait"]
  }

  provisioner "local-exec" {
    command = <<EOT
      k3sup install \
        --ssh-key "${local_sensitive_file.ssh.filename}" \
        --print-config=false \
        --ip "${self.ipv4_address}" \
        --k3s-channel stable \
        --k3s-extra-args "--disable-cloud-controller --cluster-cidr ${local.cluster_cidr} --kubelet-arg cloud-provider=external --disable=traefik --disable=servicelb --flannel-backend=none --disable=local-storage --node-external-ip ${self.ipv4_address} --node-ip ${tolist(self.network).0.ip}" \
        --local-path "${local.kubeconfig_path}"
    EOT
  }
}

resource "hcloud_server" "worker" {
  count = var.worker_count

  name        = "ccm-from-scratch-worker-${count.index}"
  server_type = "cpx11"
  location    = "fsn1"
  image       = "ubuntu-22.04"
  ssh_keys    = [hcloud_ssh_key.default.id]

  network {
    network_id = hcloud_network.cluster.id
    alias_ips = []
  }
  depends_on = [hcloud_network_subnet.cluster]

  connection {
    host        = self.ipv4_address
    user        = "root"
    type        = "ssh"
    private_key = tls_private_key.ssh.private_key_openssh
  }

  provisioner "remote-exec" {
    inline = ["cloud-init status --wait"]
  }

  provisioner "local-exec" {
    command = <<EOT
      k3sup join \
        --ssh-key "${local_sensitive_file.ssh.filename}" \
        --server-ip "${hcloud_server.cp.ipv4_address}" \
        --ip "${self.ipv4_address}" \
        --k3s-channel stable \
        --k3s-extra-args "--kubelet-arg cloud-provider=external --node-external-ip ${self.ipv4_address} --node-ip ${tolist(self.network).0.ip}"
    EOT
  }
}

data "local_sensitive_file" "kubeconfig" {
  # Kubeconfig is only written after control-plane finished
  depends_on = [hcloud_server.cp]

  filename = local.kubeconfig_path
}

provider "helm" {
  kubernetes {
    config_path = data.local_sensitive_file.kubeconfig.filename
  }
}

resource "helm_release" "cilium" {
  name       = "cilium"
  chart      = "cilium"
  repository = "https://helm.cilium.io"
  namespace  = "kube-system"
  version    = "1.13.1"

  set {
    name  = "ipam.mode"
    value = "kubernetes"
  }

  // For Routes
  set {
    name  = "tunnel"
    value = "disabled"
  }
  set {
    name = "ipv4NativeRoutingCIDR"
    value = local.cluster_cidr
  }
}

provider "kubernetes" {
  config_path = data.local_sensitive_file.kubeconfig.filename
}

resource "kubernetes_secret_v1" "hcloud_token" {
  metadata {
    name      = "hcloud"
    namespace = "kube-system"
  }

  data = {
    token = var.hcloud_token
    network = hcloud_network.cluster.id
  }
}

output "kubeconfig" {
  value = "export KUBECONFIG=${data.local_sensitive_file.kubeconfig.filename}"
}


