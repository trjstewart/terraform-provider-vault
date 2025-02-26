package vault

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/vault/api"
)

var (
	consulSecretBackendRoleBackendFromPathRegex = regexp.MustCompile("^(.+)/roles/.+$")
	consulSecretBackendRoleNameFromPathRegex    = regexp.MustCompile("^.+/roles/(.+$)")
)

func consulSecretBackendRoleResource() *schema.Resource {
	return &schema.Resource{
		Create: consulSecretBackendRoleWrite,
		Read:   consulSecretBackendRoleRead,
		Update: consulSecretBackendRoleWrite,
		Delete: consulSecretBackendRoleDelete,
		Exists: consulSecretBackendRoleExists,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of an existing role against which to create this Consul credential",
			},
			"backend": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "The path of the Consul Secret Backend the role belongs to.",
			},
			"policies": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of Consul policies to associate with this role",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"consul_roles": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: `Set of Consul roles to attach to the token. Applicable for Vault 1.10+ with Consul 1.5+`,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"consul_namespace": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				Description: "The Consul namespace that the token will be " +
					"created in. Applicable for Vault 1.10+ and Consul 1.7+",
			},
			"partition": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				Description: "The Consul admin partition that the token will be " +
					"created in. Applicable for Vault 1.10+ and Consul 1.11+",
			},
			"max_ttl": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Maximum TTL for leases associated with this role, in seconds.",
				Default:     0,
			},
			"ttl": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Specifies the TTL for this role.",
				Default:     0,
			},
			"token_type": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Specifies the type of token to create when using this role. Valid values are \"client\" or \"management\".",
				Default:     "client",
			},
			"local": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Indicates that the token should not be replicated globally and instead be local to the current datacenter.",
				Default:     false,
			},
		},
	}
}

func consulSecretBackendRoleGetBackend(d *schema.ResourceData) string {
	if v, ok := d.GetOk("backend"); ok {
		return v.(string)
	} else if v, ok := d.GetOk("path"); ok {
		return v.(string)
	} else {
		return ""
	}
}

func consulSecretBackendRoleWrite(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	name := d.Get("name").(string)

	backend := consulSecretBackendRoleGetBackend(d)
	if backend == "" {
		return fmt.Errorf("No backend specified for Consul secret backend role %s", name)
	}

	path := consulSecretBackendRolePath(backend, name)

	policies := d.Get("policies").([]interface{})
	roles := d.Get("consul_roles").(*schema.Set).List()

	if len(policies) == 0 && len(roles) == 0 {
		return fmt.Errorf("policies or consul_roles must be set")
	}

	data := map[string]interface{}{
		"policies":     policies,
		"consul_roles": roles,
	}

	params := []string{
		"max_ttl",
		"ttl",
		"token_type",
		"local",
		"consul_namespace",
		"partition",
	}
	for _, k := range params {
		if v, ok := d.GetOkExists(k); ok {
			data[k] = v
		}
	}

	log.Printf("[DEBUG] Configuring Consul secrets backend role at %q", path)

	if _, err := client.Logical().Write(path, data); err != nil {
		return fmt.Errorf("error writing role configuration for %q: %s", path, err)
	}

	d.SetId(path)
	return consulSecretBackendRoleRead(d, meta)
}

func consulSecretBackendRoleRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	upgradeOldID(d)

	path := d.Id()
	name, err := consulSecretBackendRoleNameFromPath(path)
	if err != nil {
		log.Printf("[WARN] Removing consul role %q because its ID is invalid", path)
		d.SetId("")
		return fmt.Errorf("invalid role ID %q: %s", path, err)
	}

	backend, err := consulSecretBackendRoleBackendFromPath(path)
	if err != nil {
		log.Printf("[WARN] Removing consul role %q because its ID is invalid", path)
		d.SetId("")
		return fmt.Errorf("invalid role ID %q: %s", path, err)
	}

	log.Printf("[DEBUG] Reading Consul secrets backend role at %q", path)

	secret, err := client.Logical().Read(path)
	if err != nil {
		return fmt.Errorf("error reading role configuration for %q: %s", path, err)
	}

	if secret == nil {
		return fmt.Errorf("resource not found")
	}

	data := secret.Data
	if err := d.Set("name", name); err != nil {
		return err
	}
	var pathKey string
	if _, ok := d.GetOk("path"); ok {
		pathKey = "path"
	} else {
		pathKey = "backend"
	}
	if err := d.Set(pathKey, backend); err != nil {
		return err
	}

	// map request params to schema fields
	params := map[string]string{
		"policies":         "policies",
		"max_ttl":          "max_ttl",
		"ttl":              "ttl",
		"token_type":       "token_type",
		"local":            "local",
		"consul_roles":     "consul_roles",
		"consul_namespace": "consul_namespace",
		"partition":        "partition",
	}

	for k, v := range params {
		val, ok := data[k]
		if !ok {
			switch k {
			// TODO case this by Vault version (vault-1.10+ request params)
			case "consul_roles", "consul_namespace", "partition":
				continue
			}
		}
		if err := d.Set(v, val); err != nil {
			return err
		}
	}

	return nil
}

func consulSecretBackendRoleDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	path := d.Id()

	log.Printf("[DEBUG] Deleting Consul backend role at %q", path)

	if _, err := client.Logical().Delete(path); err != nil {
		return fmt.Errorf("error deleting Consul backend role at %q: %s", path, err)
	}
	log.Printf("[DEBUG] Deleted Consul backend role at %q", path)
	return nil
}

func consulSecretBackendRoleExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(*api.Client)

	upgradeOldID(d)

	path := d.Id()

	log.Printf("[DEBUG] Checking Consul secrets backend role at %q", path)

	secret, err := client.Logical().Read(path)
	if err != nil {
		return false, fmt.Errorf("error reading role configuration for %q: %s", path, err)
	}

	return secret != nil, nil
}

func upgradeOldID(d *schema.ResourceData) {
	// Upgrade old "{backend},{name}" ID format
	id := d.Id()
	s := strings.Split(id, ",")
	if len(s) == 2 {
		backend := s[0]
		name := s[1]
		path := consulSecretBackendRolePath(backend, name)
		log.Printf("[DEBUG] Upgrading old ID %s to %s", id, path)
		d.SetId(path)
	}
}

func consulSecretBackendRolePath(backend, name string) string {
	return strings.Trim(backend, "/") + "/roles/" + name
}

func consulSecretBackendRoleNameFromPath(path string) (string, error) {
	if !consulSecretBackendRoleNameFromPathRegex.MatchString(path) {
		return "", fmt.Errorf("no name found")
	}
	res := consulSecretBackendRoleNameFromPathRegex.FindStringSubmatch(path)
	if len(res) != 2 {
		return "", fmt.Errorf("unexpected number of matches (%d) for name", len(res))
	}
	return res[1], nil
}

func consulSecretBackendRoleBackendFromPath(path string) (string, error) {
	if !consulSecretBackendRoleBackendFromPathRegex.MatchString(path) {
		return "", fmt.Errorf("no backend found")
	}
	res := consulSecretBackendRoleBackendFromPathRegex.FindStringSubmatch(path)
	if len(res) != 2 {
		return "", fmt.Errorf("unexpected number of matches (%d) for backend", len(res))
	}
	return res[1], nil
}
