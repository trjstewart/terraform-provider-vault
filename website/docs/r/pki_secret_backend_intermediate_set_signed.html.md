---
layout: "vault"
page_title: "Vault: vault_pki_secret_backend_intermediate_set_signed resource"
sidebar_current: "docs-vault-resource-pki-secret-backend-intermediate-set-signed"
description: |-
  Submit the PKI CA certificate.
---

# vault\_pki\_secret\_backend\_intermediate\_set\_signed

Submits the CA certificate to the PKI Secret Backend.

~> **Important** All data provided in the resource configuration will be
written in cleartext to state and plan files generated by Terraform, and
will appear in the console output when Terraform runs. Protect these
artifacts accordingly. See
[the main provider documentation](../index.html)
for more details.

## Example Usage

```hcl
resource "vault_mount" "root" {
  path                      = "pki-root"
  type                      = "pki"
  description               = "root"
  default_lease_ttl_seconds = 8640000
  max_lease_ttl_seconds     = 8640000
}

resource "vault_mount" "intermediate" {
  path                      = "pki-int"
  type                      = vault_mount.root.type
  description               = "intermediate"
  default_lease_ttl_seconds = 86400
  max_lease_ttl_seconds     = 86400
}

resource "vault_pki_secret_backend_root_cert" "example" {
  backend              = vault_mount.root.path
  type                 = "internal"
  common_name          = "RootOrg Root CA"
  ttl                  = 86400
  format               = "pem"
  private_key_format   = "der"
  key_type             = "rsa"
  key_bits             = 4096
  exclude_cn_from_sans = true
  ou                   = "Organizational Unit"
  organization         = "RootOrg"
  country              = "US"
  locality             = "San Francisco"
  province             = "CA"
}

resource "vault_pki_secret_backend_intermediate_cert_request" "example" {
  backend     = vault_mount.intermediate.path
  type        = vault_pki_secret_backend_root_cert.example.type
  common_name = "SubOrg Intermediate CA"
}

resource "vault_pki_secret_backend_root_sign_intermediate" "example" {
  backend              = vault_mount.root.path
  csr                  = vault_pki_secret_backend_intermediate_cert_request.example.csr
  common_name          = "SubOrg Intermediate CA"
  exclude_cn_from_sans = true
  ou                   = "SubUnit"
  organization         = "SubOrg"
  country              = "US"
  locality             = "San Francisco"
  province             = "CA"
  revoke               = true
}

resource "vault_pki_secret_backend_intermediate_set_signed" "example" {
  backend     = vault_mount.intermediate.path
  certificate = vault_pki_secret_backend_root_sign_intermediate.example.certificate
}
```

## Argument Reference

The following arguments are supported:

* `backend` - (Required) The PKI secret backend the resource belongs to.

* `certificate` - (Required) Specifies the PEM encoded certificate. May optionally append additional
  CA certificates to populate the whole chain, which will then enable returning the full chain from
  issue and sign operations.

## Attributes Reference

No additional attributes are exported by this resource.
