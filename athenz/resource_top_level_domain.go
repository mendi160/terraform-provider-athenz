package athenz

import (
	"fmt"
	"log"

	"github.com/AthenZ/athenz/clients/go/zms"
	"github.com/AthenZ/terraform-provider-athenz/client"
	"github.com/ardielle/ardielle-go/rdl"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func ResourceTopLevelDomain() *schema.Resource {
	return &schema.Resource{
		Create: resourceTopLevelDomainCreate,
		Read:   resourceTopLevelDomainRead,
		Delete: resourceTopLevelDomainDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Description: "Name of the standard Top Level domain",
				Required:    true,
				ForceNew:    true,
			},
			"audit_ref": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true, // must to be true, because no update method
				Default:  AUDIT_REF,
			},
			"admin_users": {
				Type:        schema.TypeSet,
				Description: "Names of the standard admin users",
				Required:    true,
				ForceNew:    true, // must to be true, because no update method
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"ypm_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true, // must to be true, because no update method
			},
		},
	}
}

func resourceTopLevelDomainCreate(d *schema.ResourceData, meta interface{}) error {
	zmsClient := meta.(client.ZmsClient)
	domainName := d.Get("name").(string)
	auditRef := d.Get("audit_ref").(string)
	adminUsers := d.Get("admin_users").(*schema.Set).List()
	ypmId := int32(d.Get("ypm_id").(int))
	topLevelDomainDetail := zms.TopLevelDomain{
		Name:       zms.SimpleName(domainName),
		AdminUsers: convertToZmsResourceNameList(adminUsers),
		YpmId:      &ypmId,
	}
	topLevelDomain, err := zmsClient.PostTopLevelDomain(auditRef, &topLevelDomainDetail)
	if err != nil {
		return err
	}
	if topLevelDomain == nil {
		return fmt.Errorf("error creating Top Level Domain: %s", err)
	}
	d.SetId(domainName)
	return resourceTopLevelDomainRead(d, meta)
}

func resourceTopLevelDomainRead(d *schema.ResourceData, meta interface{}) error {
	zmsClient := meta.(client.ZmsClient)
	domainName := d.Id()
	topLevelDomain, err := zmsClient.GetDomain(domainName)
	switch v := err.(type) {
	case rdl.ResourceError:
		if v.Code == 404 {
			log.Printf("[WARN] Athenz Top Level Domain %s not found, removing from state", d.Id())
			d.SetId("")
			return nil
		}
		return fmt.Errorf("error retrieving Athenz Top level Domain: %s", v)
	case rdl.Any:
		return err
	}

	if topLevelDomain == nil {
		return fmt.Errorf("error retrieving Athenz Top Level Domain - Make sure your cert/key are valid")
	}
	if err = d.Set("name", domainName); err != nil {
		return err
	}
	adminRole, err := zmsClient.GetRole(domainName, "admin")
	if err != nil {
		return err
	}
	adminUsers := flattenRoleMembers(adminRole.RoleMembers)
	if err = d.Set("admin_users", adminUsers); err != nil {
		return err
	}
	if err = d.Set("ypm_id", int(*topLevelDomain.YpmId)); err != nil {
		return err
	}
	return nil
}

func resourceTopLevelDomainDelete(d *schema.ResourceData, meta interface{}) error {
	zmsClient := meta.(client.ZmsClient)
	domainName := d.Id()
	auditRef := d.Get("audit_ref").(string)
	err := zmsClient.DeleteTopLevelDomain(domainName, auditRef)
	if err != nil {
		return err
	}
	return nil
}
