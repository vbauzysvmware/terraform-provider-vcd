package vcd

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/vmware/go-vcloud-director/v2/govcd"
	"github.com/vmware/go-vcloud-director/v2/types/v56"
)

func resourceVcdVmSizingPolicy() *schema.Resource {

	return &schema.Resource{
		Create: resourceVmSizingPolicyCreate,
		Delete: resourceVmSizingPolicyDelete,
		Read:   resourceVmSizingPolicyRead,
		Update: resourceVmSizingPolicyUpdate,
		Importer: &schema.ResourceImporter{
			State: resourceVmSizingPolicyImport,
		},
		Schema: map[string]*schema.Schema{
			"org": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Description: "The name of organization to use, optional if defined at provider " +
					"level. Useful when connected as sysadmin working across different organizations",
			},
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"description": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"cpu": &schema.Schema{
				Optional: true,
				MinItems: 0,
				MaxItems: 1,
				Type:     schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"speed_in_mhz": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "Defines the vCPU speed of a core in MHz.",
							ValidateFunc: IsIntAndAtLeast(0),
						},
						"count": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "Defines the number of vCPUs configured for a VM. This is a VM hardware configuration. When a tenant assigns the VM sizing policy to a VM, this count becomes the configured number of vCPUs for the VM.",
							ValidateFunc: IsIntAndAtLeast(0),
						},
						"cores_per_socket": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "The number of cores per socket for a VM. This is a VM hardware configuration. The number of vCPUs that is defined in the VM sizing policy must be divisible by the number of cores per socket. If the number of vCPUs is not divisible by the number of cores per socket, the number of cores per socket becomes invalid.",
							ValidateFunc: IsIntAndAtLeast(0),
						},
						"reservation_guarantee": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "Defines how much of the CPU resources of a VM are reserved. The allocated CPU for a VM equals the number of vCPUs times the vCPU speed in MHz. The value of the attribute ranges between 0 and one. Value of 0 CPU reservation guarantee defines no CPU reservation. Value of 1 defines 100% of CPU reserved.",
							ValidateFunc: IsFloatAndBetween(0, 1),
						},
						"limit_in_mhz": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "Defines the CPU limit in MHz for a VM. If not defined in the VDC compute policy, CPU limit is equal to the vCPU speed multiplied by the number of vCPUs.",
							ValidateFunc: IsIntAndAtLeast(0),
						},
						"shares": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "Defines the number of CPU shares for a VM. Shares specify the relative importance of a VM within a virtual data center. If a VM has twice as many shares of CPU as another VM, it is entitled to consume twice as much CPU when these two virtual machines are competing for resources. If not defined in the VDC compute policy, normal shares are applied to the VM.",
							ValidateFunc: IsIntAndAtLeast(0),
						},
					},
				},
			},
			"memory": &schema.Schema{
				Optional: true,
				MinItems: 0,
				MaxItems: 1,
				Type:     schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"size_in_mb": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "Defines the memory configured for a VM in MB. This is a VM hardware configuration. When a tenant assigns the VM sizing policy to a VM, the VM receives the amount of memory defined by this attribute.",
							ValidateFunc: IsIntAndAtLeast(0),
						},
						"reservation_guarantee": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "Defines the reserved amount of memory that is configured for a VM. The value of the attribute ranges between 0 and one. Value of 0 memory reservation guarantee defines no memory reservation. Value of 1 defines 100% of memory reserved.",
							ValidateFunc: IsFloatAndBetween(0, 1),
						},
						"limit_in_mb": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "Defines the memory limit in MB for a VM. If not defined in the VM sizing policy, memory limit is equal to the allocated memory for the VM.",
							ValidateFunc: IsIntAndAtLeast(0),
						},
						"shares": {
							Type:         schema.TypeString,
							Optional:     true,
							Description:  "Defines the number of memory shares for a VM. Shares specify the relative importance of a VM within a virtual data center. If a VM has twice as many shares of memory as another VM, it is entitled to consume twice as much memory when these two virtual machines are competing for resources. If not defined in the VDC compute policy, normal shares are applied to the VM.",
							ValidateFunc: IsIntAndAtLeast(0),
						},
					},
				},
			},
		},
	}
}

func resourceVmSizingPolicyCreate(d *schema.ResourceData, meta interface{}) error {
	policyName := d.Get("name").(string)
	log.Printf("[TRACE] VM sizing policy creation initiated: %s", policyName)

	vcdClient := meta.(*VCDClient)

	if !vcdClient.Client.IsSysAdmin {
		return fmt.Errorf("functionality requires system administrator privileges")
	}

	adminOrg, err := vcdClient.GetAdminOrgFromResource(d)
	if err != nil {
		return fmt.Errorf(errorRetrievingOrg, err)
	}

	params, err := getVmZingingPolicyInput(d, vcdClient)
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Creating VM sizing policy: %#v", params)

	createdVmSizingPolicy, err := adminOrg.CreateVdcComputePolicy(params)
	if err != nil {
		log.Printf("[DEBUG] Error VM sizing policy: %s", err)
		return fmt.Errorf("error VM sizing policy: %s", err)
	}

	d.SetId(createdVmSizingPolicy.VdcComputePolicy.ID)
	log.Printf("[TRACE] VM sizing policy created: %#v", createdVmSizingPolicy.VdcComputePolicy)

	return resourceVmSizingPolicyRead(d, meta)
}

// Fetches information about an existing VM sizing policy for a data definition
func resourceVmSizingPolicyRead(d *schema.ResourceData, meta interface{}) error {
	policyName := d.Get("name").(string)
	log.Printf("[TRACE] VM sizing policy read initiated: %s", policyName)

	vcdClient := meta.(*VCDClient)

	adminOrg, err := vcdClient.GetAdminOrgFromResource(d)
	if err != nil {
		return fmt.Errorf(errorRetrievingOrg, err)
	}

	policy, err := adminOrg.GetVdcComputePolicyById(d.Id())
	if err != nil {
		log.Printf("[DEBUG] Unable to find VM sizing policy %s", policyName)
		return fmt.Errorf("unable to find VM sizing policy %s, err: %s", policyName, err)
	}

	return setVmSizingPolicy(d, policy)
}

// setVmSizingPolicy sets object state from *govcd.VdcComputePolicy
func setVmSizingPolicy(d *schema.ResourceData, policy *govcd.VdcComputePolicy) error {

	_ = d.Set("name", policy.VdcComputePolicy.Name)
	_ = d.Set("description", policy.VdcComputePolicy.Description)

	var cpuList []map[string]interface{}
	cpuMap := make(map[string]interface{})

	if policy.VdcComputePolicy.CPUShares != nil {
		cpuMap["shares"] = policy.VdcComputePolicy.CPUShares
	}
	cpuFieldProvided := false
	if policy.VdcComputePolicy.CPUShares != nil {
		cpuMap["shares"] = strconv.Itoa(*policy.VdcComputePolicy.CPUShares)
		cpuFieldProvided = true
	}
	if policy.VdcComputePolicy.CPULimit != nil {
		cpuMap["limit_in_mhz"] = strconv.Itoa(*policy.VdcComputePolicy.CPULimit)
		cpuFieldProvided = true
	}
	if policy.VdcComputePolicy.CPUCount != nil {
		cpuMap["count"] = strconv.Itoa(*policy.VdcComputePolicy.CPUCount)
		cpuFieldProvided = true
	}
	if policy.VdcComputePolicy.CPUSpeed != nil {
		cpuMap["speed_in_mhz"] = strconv.Itoa(*policy.VdcComputePolicy.CPUSpeed)
		cpuFieldProvided = true
	}
	if policy.VdcComputePolicy.CoresPerSocket != nil {
		cpuMap["cores_per_socket"] = strconv.Itoa(*policy.VdcComputePolicy.CoresPerSocket)
		cpuFieldProvided = true
	}
	if policy.VdcComputePolicy.CPUReservationGuarantee != nil {
		cpuMap["reservation_guarantee"] = strconv.FormatFloat(*policy.VdcComputePolicy.CPUReservationGuarantee, 'f', -1, 64)
		cpuFieldProvided = true
	}
	if cpuFieldProvided {
		cpuList = append(cpuList, cpuMap)
		err := d.Set("cpu", cpuList)
		if err != nil {
			return err
		}
	}

	var memoryList []map[string]interface{}
	memoryMap := make(map[string]interface{})
	memoryFieldProvided := false
	if policy.VdcComputePolicy.Memory != nil {
		memoryMap["size_in_mb"] = strconv.Itoa(*policy.VdcComputePolicy.Memory)
		memoryFieldProvided = true
	}
	if policy.VdcComputePolicy.MemoryLimit != nil {
		memoryMap["limit_in_mb"] = strconv.Itoa(*policy.VdcComputePolicy.MemoryLimit)
		memoryFieldProvided = true
	}
	if policy.VdcComputePolicy.MemoryShares != nil {
		memoryMap["shares"] = strconv.Itoa(*policy.VdcComputePolicy.MemoryShares)
		memoryFieldProvided = true
	}
	if policy.VdcComputePolicy.MemoryReservationGuarantee != nil {
		memoryMap["reservation_guarantee"] = strconv.FormatFloat(*policy.VdcComputePolicy.MemoryReservationGuarantee, 'f', -1, 64)
		memoryFieldProvided = true
	}
	if memoryFieldProvided {
		memoryList = append(memoryList, memoryMap)
		err := d.Set("memory", memoryList)
		if err != nil {
			return err
		}
	}

	log.Printf("[TRACE] VM sizing policy read completed: %#v", policy.VdcComputePolicy.Name)
	return nil
}

//resourceVmSizingPolicyUpdate function updates resource with found configurations changes
func resourceVmSizingPolicyUpdate(d *schema.ResourceData, meta interface{}) error {
	policyName := d.Get("name").(string)
	log.Printf("[TRACE] VM sizing policy update initiated: %s", policyName)

	vcdClient := meta.(*VCDClient)

	adminOrg, err := vcdClient.GetAdminOrgFromResource(d)
	if err != nil {
		return fmt.Errorf(errorRetrievingOrg, err)
	}

	policy, err := adminOrg.GetVdcComputePolicyById(d.Id())
	if err != nil {
		log.Printf("[DEBUG] Unable to find VM sizing policy %s", policyName)
		return fmt.Errorf("unable to find VM sizing policy %s, error:  %s", policyName, err)
	}

	changedPolicy, err := getUpdatedVmSizingPolicyInput(d, policy)
	if err != nil {
		log.Printf("[DEBUG] Error updating VM sizing policy %s with error %s", policyName, err)
		return fmt.Errorf("error updating VM sizing policy %s, err: %s", policyName, err)
	}

	_, err = changedPolicy.Update()
	if err != nil {
		log.Printf("[DEBUG] Error updating VM sizing policy %s with error %s", policyName, err)
		return fmt.Errorf("error updating VM sizing policy %s, err: %s", policyName, err)
	}

	log.Printf("[TRACE] VM sizing policy update completed: %s", policyName)
	return resourceVmSizingPolicyRead(d, meta)
}

// Deletes a VM sizing policy
func resourceVmSizingPolicyDelete(d *schema.ResourceData, meta interface{}) error {
	policyName := d.Get("name").(string)
	log.Printf("[TRACE] VM sizing policy delete started: %s", policyName)

	vcdClient := meta.(*VCDClient)

	if !vcdClient.Client.IsSysAdmin {
		return fmt.Errorf("functionality requires system administrator privileges")
	}

	adminOrg, err := vcdClient.GetAdminOrgFromResource(d)
	if err != nil {
		return fmt.Errorf(errorRetrievingOrg, err)
	}

	policy, err := adminOrg.GetVdcComputePolicyById(d.Id())
	if err != nil {
		log.Printf("[DEBUG] Unable to find VM sizing policy %s. Removing from tfstate", policyName)
		d.SetId("")
		return nil
	}

	err = policy.Delete()
	if err != nil {
		log.Printf("[DEBUG] Error removing VM sizing policy %s, err: %s", policyName, err)
		return fmt.Errorf("error removing VM sizing policy %s, err: %s", policyName, err)
	}

	log.Printf("[TRACE] VM sizing policy delete completed: %s", policyName)
	return nil
}

// helper for transforming the resource input into the VdcComputePolicy structure
func getUpdatedVmSizingPolicyInput(d *schema.ResourceData, policy *govcd.VdcComputePolicy) (*govcd.VdcComputePolicy, error) {
	if d.HasChange("name") {
		policy.VdcComputePolicy.Name = d.Get("name").(string)
	}

	if d.HasChange("description") {
		policy.VdcComputePolicy.Description = d.Get("description").(string)
	}

	if d.HasChange("cpu") {
		return nil, fmt.Errorf("only name and description are updatable for VM sizing policy")
	}
	if d.HasChange("memory") {
		return nil, fmt.Errorf("only name and description are updatable for VM sizing policy")
	}

	return policy, nil
}

// helper for transforming the resource input into the VdcComputePolicy structure
// any cast operations or default values should be done here so that the create method is simple
func getVmZingingPolicyInput(d *schema.ResourceData, vcdClient *VCDClient) (*types.VdcComputePolicy, error) {

	params := &types.VdcComputePolicy{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
	}

	cpuPart := d.Get("cpu").([]interface{})
	if len(cpuPart) == 1 {
		var err error
		params, err = getCpuInput(cpuPart, params)
		if err != nil {
			return nil, err
		}
	}

	memoryPart := d.Get("memory").([]interface{})
	if len(memoryPart) == 1 {
		var err error
		params, err = getMemoryInput(memoryPart, params)
		if err != nil {
			return nil, err
		}
	}
	return params, nil
}

func getCpuInput(cpuPart []interface{}, params *types.VdcComputePolicy) (*types.VdcComputePolicy, error) {
	cpuMap := cpuPart[0].(map[string]interface{})

	speedInMhz := cpuMap["speed_in_mhz"].(string)
	if speedInMhz != "" {
		convertedNumber, err := strconv.Atoi(speedInMhz)
		if err != nil {
			return nil, fmt.Errorf("value `%s` speed_in_mhz is not number. err: %s", speedInMhz, err)
		}
		params.CPUSpeed = &convertedNumber
	}
	limitInMhz := cpuMap["limit_in_mhz"].(string)
	if limitInMhz != "" {
		convertedNumber, err := strconv.Atoi(limitInMhz)
		if err != nil {
			return nil, fmt.Errorf("value `%s` limit_in_mhz is not number. err: %s", limitInMhz, err)
		}
		params.CPULimit = &convertedNumber
	}
	shares := cpuMap["shares"].(string)
	if shares != "" {
		convertedNumber, err := strconv.Atoi(shares)
		if err != nil {
			return nil, fmt.Errorf("value `%s` shares is not number. err: %s", shares, err)
		}
		params.CPUShares = &convertedNumber
	}
	count := cpuMap["count"].(string)
	if shares != "" {
		convertedNumber, err := strconv.Atoi(count)
		if err != nil {
			return nil, fmt.Errorf("value `%s` count is not number. err: %s", count, err)
		}
		params.CPUCount = &convertedNumber
	}
	coresPerSocket := cpuMap["cores_per_socket"].(string)
	if shares != "" {
		convertedNumber, err := strconv.Atoi(coresPerSocket)
		if err != nil {
			return nil, fmt.Errorf("value `%s` cores_per_socket is not number. err: %s", coresPerSocket, err)
		}
		params.CoresPerSocket = &convertedNumber
	}
	cpuReservation := cpuMap["reservation_guarantee"].(string)
	if cpuReservation != "" {
		convertedNumber, err := strconv.ParseFloat(cpuReservation, 64)
		if err != nil {
			return nil, fmt.Errorf("value `%s` reservation_guarantee is not number. err: %s", cpuReservation, err)
		}
		params.CPUReservationGuarantee = &convertedNumber
	}
	return params, nil
}

func getMemoryInput(memoryPart []interface{}, params *types.VdcComputePolicy) (*types.VdcComputePolicy, error) {
	memoryMap := memoryPart[0].(map[string]interface{})
	sizeInMb := memoryMap["size_in_mb"].(string)
	if sizeInMb != "" {
		convertedNumber, err := strconv.Atoi(sizeInMb)
		if err != nil {
			return nil, fmt.Errorf("value `%s` size_in_mb is not number. err: %s", sizeInMb, err)
		}
		params.Memory = &convertedNumber
	}
	limitInMb := memoryMap["limit_in_mb"].(string)
	if limitInMb != "" {
		convertedNumber, err := strconv.Atoi(limitInMb)
		if err != nil {
			return nil, fmt.Errorf("value `%s` limit_in_mb is not number. err: %s", limitInMb, err)
		}
		params.MemoryLimit = &convertedNumber
	}
	shares := memoryMap["shares"].(string)
	if shares != "" {
		convertedNumber, err := strconv.Atoi(shares)
		if err != nil {
			return nil, fmt.Errorf("value `%s` shares is not number. err: %s", shares, err)
		}
		params.MemoryShares = &convertedNumber
	}
	memoryReservation := memoryMap["reservation_guarantee"].(string)
	if memoryReservation != "" {
		convertedNumber, err := strconv.ParseFloat(memoryReservation, 64)
		if err != nil {
			return nil, fmt.Errorf("value `%s` reservation_guarantee is not number. err: %s", memoryReservation, err)
		}
		params.MemoryReservationGuarantee = &convertedNumber
	}
	return params, nil
}

// resourceVmSizingPolicyImport is responsible for importing the resource.
// The following steps happen as part of import
// 1. The user supplies `terraform import _resource_name_ _the_id_string_` command
// 2. `_the_id_string_` contains a dot formatted path to resource as in the example below
// 3. The functions splits the dot-formatted path and tries to lookup the object
// 4. If the lookup succeeds it set's the ID field for `_resource_name_` resource in state file
// (the resource must be already defined in .tf config otherwise `terraform import` will complain)
// 5. `terraform refresh` is being implicitly launched. The Read method looks up all other fields
// based on the known ID of object.
//
// Example resource name (_resource_name_): vcd_vm_sizing_policy.my_existing_policy_name
// Example import path (_the_id_string_): org.my_existing_vm_sizing_policy_id
// Note: the separator can be changed using Provider.import_separator or variable VCD_IMPORT_SEPARATOR
func resourceVmSizingPolicyImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	resourceURI := strings.Split(d.Id(), ImportSeparator)
	if len(resourceURI) != 2 {
		return nil, fmt.Errorf("resource name must be specified as org.my_existing_vm_sizing_policy_id")
	}
	orgName, policyId := resourceURI[0], resourceURI[1]

	vcdClient := meta.(*VCDClient)

	adminOrg, err := vcdClient.GetAdminOrg(orgName)
	if err != nil {
		return nil, fmt.Errorf(errorRetrievingOrg, err)
	}

	vmSizingPolicy, err := adminOrg.GetVdcComputePolicyById(policyId)
	if err != nil {
		log.Printf("[DEBUG] Unable to find VM sizing policy %s", policyId)
		return nil, fmt.Errorf("unable to find VM sizing policy %s, err: %s", policyId, err)
	}

	_ = d.Set("org", orgName)
	_ = d.Set("name", vmSizingPolicy.VdcComputePolicy.Name)

	d.SetId(vmSizingPolicy.VdcComputePolicy.ID)

	return []*schema.ResourceData{d}, nil
}
