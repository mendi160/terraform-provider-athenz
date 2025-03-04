package athenz

import (
	"fmt"
	"log"
	"strings"

	"github.com/AthenZ/athenz/clients/go/zms"

	"github.com/AthenZ/terraform-provider-athenz/client"

	"github.com/ardielle/ardielle-go/rdl"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func ResourceRole() *schema.Resource {
	return &schema.Resource{
		Create: resourceRoleCreate,
		Read:   resourceRoleRead,
		Update: resourceRoleUpdate,
		Delete: resourceRoleDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"domain": {
				Type:        schema.TypeString,
				Description: "Name of the domain that role belongs to",
				Required:    true,
				ForceNew:    true,
			},
			"name": {
				Type:        schema.TypeString,
				Description: "Name of the standard group role",
				Required:    true,
				ForceNew:    true,
			},
			"members": {
				Type:        schema.TypeSet,
				Description: "Users or services to be added as members",
				Optional:    true,
				Computed:    false,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Set:         schema.HashString,
			},
			"audit_ref": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  AUDIT_REF,
			},
			"tags": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceRoleCreate(d *schema.ResourceData, meta interface{}) error {
	zmsClient := meta.(client.ZmsClient)
	dn := d.Get("domain").(string)
	rn := d.Get("name").(string)
	fullResourceName := dn + ROLE_SEPARATOR + rn

	roleCheck, err := zmsClient.GetRole(dn, rn)
	switch v := err.(type) {
	case rdl.ResourceError:
		if v.Code == 404 {
			role := zms.Role{
				Name:     zms.ResourceName(fullResourceName),
				Modified: nil,
			}
			if v, ok := d.GetOk("members"); ok && v.(*schema.Set).Len() > 0 {
				role.RoleMembers = expandRoleMembers(v.(*schema.Set).List())
			}
			auditRef := d.Get("audit_ref").(string)
			if v, ok := d.GetOk("tags"); ok {
				role.Tags = expandRoleTags(v.(map[string]interface{}))
			}
			err = zmsClient.PutRole(dn, rn, auditRef, &role)
			if err != nil {
				return err
			}
		}
	case rdl.Any:
		return err
	case nil:
		if roleCheck != nil {
			return fmt.Errorf("the role %s is already exists in the domain %s use terraform import command", rn, dn)
		} else {
			return err
		}
	}
	d.SetId(fullResourceName)

	return resourceRoleRead(d, meta)
}

func resourceRoleRead(d *schema.ResourceData, meta interface{}) error {
	zmsClient := meta.(client.ZmsClient)

	fullResourceName := strings.Split(d.Id(), ROLE_SEPARATOR)
	dn, rn := fullResourceName[0], fullResourceName[1]
	if err := d.Set("domain", dn); err != nil {
		return err
	}
	if err := d.Set("name", rn); err != nil {
		return err
	}
	role, err := zmsClient.GetRole(dn, rn)
	switch v := err.(type) {
	case rdl.ResourceError:
		if v.Code == 404 {
			log.Printf("[WARN] Athenz Role %s not found, removing from state", d.Id())
			d.SetId("")
			return nil
		}
		return fmt.Errorf("error retrieving Athenz Role %s: %s", d.Id(), v)
	case rdl.Any:
		return err
	}

	if role == nil {
		return fmt.Errorf("error retrieving Athenz Role - Make sure your cert/key are valid")
	}

	if len(role.RoleMembers) > 0 {
		if err = d.Set("members", flattenRoleMembers(role.RoleMembers)); err != nil {
			return err
		}
	}
	// added for role tag
	if len(role.Tags) > 0 {
		if err = d.Set("tags", flattenTag(role.Tags)); err != nil {
			return err
		}
	}

	return nil
}

func resourceRoleUpdate(d *schema.ResourceData, meta interface{}) error {
	zmsClient := meta.(client.ZmsClient)
	fullResourceName := strings.Split(d.Id(), ROLE_SEPARATOR)
	dn, rn := fullResourceName[0], fullResourceName[1]
	auditRef := d.Get("audit_ref").(string)
	if d.HasChange("members") {
		os, ns := handleChange(d, "members")
		remove := expandRoleMembers(os.Difference(ns).List())
		add := expandRoleMembers(ns.Difference(os).List())
		err := updateRoleMembers(dn, rn, remove, add, auditRef, zmsClient)
		if err != nil {
			return fmt.Errorf("error updating group membership: %s", err)
		}
	}
	if d.HasChange("tags") {
		role, err := zmsClient.GetRole(dn, rn)
		if err != nil {
			return err
		}
		_, n := d.GetChange("tags")
		tags := expandRoleTags(n.(map[string]interface{}))
		role.Tags = tags
		err = zmsClient.PutRole(dn, rn, auditRef, role)
		if err != nil {
			return fmt.Errorf("error updating tags: %s", err)
		}
	}
	return resourceRoleRead(d, meta)
}

func resourceRoleDelete(d *schema.ResourceData, meta interface{}) error {
	zmsClient := meta.(client.ZmsClient)
	fullResourceName := strings.Split(d.Id(), ROLE_SEPARATOR)
	dn, rn := fullResourceName[0], fullResourceName[1]
	auditRef := d.Get("audit_ref").(string)
	err := zmsClient.DeleteRole(dn, rn, auditRef)
	if err != nil {
		return err
	}

	return nil
}
