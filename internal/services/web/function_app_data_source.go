package web

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-provider-azurerm/helpers/azure"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/web/parse"
	webValidate "github.com/hashicorp/terraform-provider-azurerm/internal/services/web/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tags"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

func dataSourceFunctionApp() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Read: dataSourceFunctionAppRead,

		Timeouts: &pluginsdk.ResourceTimeout{
			Read: pluginsdk.DefaultTimeout(5 * time.Minute),
		},

		Schema: map[string]*pluginsdk.Schema{
			"name": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ValidateFunc: webValidate.AppServiceName,
			},

			"resource_group_name": azure.SchemaResourceGroupNameForDataSource(),

			"location": azure.SchemaLocationForDataSource(),

			"app_service_plan_id": {
				Type:     pluginsdk.TypeString,
				Computed: true,
			},

			"app_settings": {
				Type:     pluginsdk.TypeMap,
				Computed: true,
				Elem: &pluginsdk.Schema{
					Type: pluginsdk.TypeString,
				},
			},

			"connection_string": {
				Type:     pluginsdk.TypeList,
				Computed: true,
				Elem: &pluginsdk.Resource{
					Schema: map[string]*pluginsdk.Schema{
						"name": {
							Type:     pluginsdk.TypeString,
							Computed: true,
						},
						"value": {
							Type:      pluginsdk.TypeString,
							Sensitive: true,
							Computed:  true,
						},
						"type": {
							Type:     pluginsdk.TypeString,
							Computed: true,
						},
					},
				},
			},

			"custom_domain_verification_id": {
				Type:     pluginsdk.TypeString,
				Computed: true,
			},

			"default_hostname": {
				Type:     pluginsdk.TypeString,
				Computed: true,
			},

			"enabled": {
				Type:     pluginsdk.TypeBool,
				Computed: true,
			},

			"os_type": {
				Type:     pluginsdk.TypeString,
				Computed: true,
			},

			"client_cert_mode": {
				Type:     pluginsdk.TypeString,
				Computed: true,
			},

			"outbound_ip_addresses": {
				Type:     pluginsdk.TypeString,
				Computed: true,
			},

			"possible_outbound_ip_addresses": {
				Type:     pluginsdk.TypeString,
				Computed: true,
			},

			"site_credential": {
				Type:     pluginsdk.TypeList,
				Computed: true,
				Elem: &pluginsdk.Resource{
					Schema: map[string]*pluginsdk.Schema{
						"username": {
							Type:     pluginsdk.TypeString,
							Computed: true,
						},
						"password": {
							Type:      pluginsdk.TypeString,
							Computed:  true,
							Sensitive: true,
						},
					},
				},
			},

			"site_config": schemaFunctionAppDataSourceSiteConfig(),

			"source_control": schemaAppServiceSiteSourceControlDataSource(),

			"identity": {
				Type:     pluginsdk.TypeList,
				Computed: true,
				Elem: &pluginsdk.Resource{
					Schema: map[string]*pluginsdk.Schema{
						"type": {
							Type:     pluginsdk.TypeString,
							Computed: true,
						},

						"principal_id": {
							Type:     pluginsdk.TypeString,
							Computed: true,
						},

						"tenant_id": {
							Type:     pluginsdk.TypeString,
							Computed: true,
						},
					},
				},
			},

			"tags": tags.Schema(),
		},
	}
}

func dataSourceFunctionAppRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Web.AppServicesClient
	subscriptionId := meta.(*clients.Client).Account.SubscriptionId
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id := parse.NewFunctionAppID(subscriptionId, d.Get("resource_group_name").(string), d.Get("name").(string))

	resp, err := client.Get(ctx, id.ResourceGroup, id.SiteName)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			return fmt.Errorf("%s was not found", id)
		}
		return fmt.Errorf("making Read request on %s: %+v", id, err)
	}

	appSettingsResp, err := client.ListApplicationSettings(ctx, id.ResourceGroup, id.SiteName)
	if err != nil {
		if utils.ResponseWasNotFound(appSettingsResp.Response) {
			return fmt.Errorf("%s Application Settings was not found", id)
		}
		return fmt.Errorf("making Read request on %s AppSettings: %+v", id, err)
	}

	connectionStringsResp, err := client.ListConnectionStrings(ctx, id.ResourceGroup, id.SiteName)
	if err != nil {
		return fmt.Errorf("making Read request on %s ConnectionStrings: %+v", id, err)
	}

	scmResp, err := client.GetSourceControl(ctx, id.ResourceGroup, id.SiteName)
	if err != nil {
		return fmt.Errorf("making Read request on %s Source Control: %+v", id, err)
	}

	siteCredFuture, err := client.ListPublishingCredentials(ctx, id.ResourceGroup, id.SiteName)
	if err != nil {
		return err
	}
	err = siteCredFuture.WaitForCompletionRef(ctx, client.Client)
	if err != nil {
		return err
	}
	siteCredResp, err := siteCredFuture.Result(*client)
	if err != nil {
		return fmt.Errorf("making Read request on %s Site Credential: %+v", id, err)
	}
	configResp, err := client.GetConfiguration(ctx, id.ResourceGroup, id.SiteName)
	if err != nil {
		return fmt.Errorf("making Read request on %s Configuration: %+v", id, err)
	}

	d.SetId(id.ID())

	d.Set("name", id.SiteName)
	d.Set("resource_group_name", id.ResourceGroup)

	if location := resp.Location; location != nil {
		d.Set("location", azure.NormalizeLocation(*location))
	}

	if props := resp.SiteProperties; props != nil {
		d.Set("app_service_plan_id", props.ServerFarmID)
		d.Set("enabled", props.Enabled)
		d.Set("default_hostname", props.DefaultHostName)
		d.Set("outbound_ip_addresses", props.OutboundIPAddresses)
		d.Set("possible_outbound_ip_addresses", props.PossibleOutboundIPAddresses)
		d.Set("custom_domain_verification_id", props.CustomDomainVerificationID)

		clientCertMode := ""
		if props.ClientCertEnabled != nil && *props.ClientCertEnabled {
			clientCertMode = string(props.ClientCertMode)
		}
		d.Set("client_cert_mode", clientCertMode)
	}

	osType := ""
	if v := resp.Kind; v != nil && strings.Contains(*v, "linux") {
		osType = "linux"
	}
	d.Set("os_type", osType)

	appSettings := flattenAppServiceAppSettings(appSettingsResp.Properties)

	if err = d.Set("app_settings", appSettings); err != nil {
		return err
	}

	if err = d.Set("connection_string", flattenFunctionAppConnectionStrings(connectionStringsResp.Properties)); err != nil {
		return err
	}

	if err := d.Set("identity", flattenFunctionAppIdentity(resp.Identity)); err != nil {
		return fmt.Errorf("setting `identity`: %+v", err)
	}

	siteCred := flattenFunctionAppSiteCredential(siteCredResp.UserProperties)
	if err = d.Set("site_credential", siteCred); err != nil {
		return err
	}

	siteConfig := flattenFunctionAppSiteConfig(configResp.SiteConfig)
	if err = d.Set("site_config", siteConfig); err != nil {
		return err
	}

	scm := flattenAppServiceSourceControl(scmResp.SiteSourceControlProperties)
	if err := d.Set("source_control", scm); err != nil {
		return err
	}

	return tags.FlattenAndSet(d, resp.Tags)
}
