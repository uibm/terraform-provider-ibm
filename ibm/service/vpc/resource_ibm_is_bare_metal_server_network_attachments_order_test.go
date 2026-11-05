// Copyright IBM Corp. 2025 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package vpc_test

// These tests replicate the false drift reported on ibm_is_bare_metal_server
// when the VPC API returns the server's network_attachments in a different
// order on every GET. They run against the VPC API endpoint configured through
// IBMCLOUD_IS_NG_API_ENDPOINT (for example the local IBM Cloud mock servers,
// see https://github.com/ramuklawjju/ibm-mock-servers) through a small
// reverse proxy that rewrites the order of "network_attachments" in the
// GET /bare_metal_servers/{id} response. The proxy lets one test step see
// the attachments in the order they were created and the next step see them
// shuffled, reversed or rotated, exactly like the real API behaves today.
//
// Run with the mocks (ports from the mock repo's catalog/PORTS.md):
//
//	IC_API_KEY=mock IAAS_CLASSIC_API_KEY=mock IAAS_CLASSIC_USERNAME=mock \
//	IBMCLOUD_IAM_API_ENDPOINT=http://mock.localhost:8168 \
//	IBMCLOUD_IS_NG_API_ENDPOINT=http://mock.localhost:8228/v1 \
//	TF_ACC=1 go test ./ibm/service/vpc -run 'TestAccIBMISBareMetalServer_networkAttachmentsOrder' -v

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	acc "github.com/IBM-Cloud/terraform-provider-ibm/ibm/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// Orders the proxy can apply to the network_attachments array of a
// GET /bare_metal_servers/{id} response.
const (
	bmsAttachmentOrderSame    = "same"    // as the backend returned them (creation order on the mocks)
	bmsAttachmentOrderReverse = "reverse" // last attachment first
	bmsAttachmentOrderRotate  = "rotate"  // shift by one more position on every call
	bmsAttachmentOrderShuffle = "shuffle" // seeded pseudo random order, different on every call
)

var bmsGetPathRe = regexp.MustCompile(`^/v1/bare_metal_servers/[^/]+$`)

// bmsAttachmentOrderProxy sits between the provider and the VPC API and
// reorders "network_attachments" in bare metal server GET responses.
type bmsAttachmentOrderProxy struct {
	server *httptest.Server
	mu     sync.Mutex
	mode   string
	calls  int
	rng    *rand.Rand
	// orders records the attachment names in the order the provider saw them, per GET.
	orders [][]string
}

func (p *bmsAttachmentOrderProxy) setMode(mode string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = mode
	p.calls = 0
}

func (p *bmsAttachmentOrderProxy) allOrders() [][]string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]string, len(p.orders))
	copy(out, p.orders)
	return out
}

func (p *bmsAttachmentOrderProxy) reorder(items []interface{}) []interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	n := len(items)
	out := make([]interface{}, n)
	copy(out, items)
	switch p.mode {
	case bmsAttachmentOrderReverse:
		for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	case bmsAttachmentOrderRotate:
		if n > 1 {
			shift := p.calls % n
			out = append(items[shift:n:n], items[:shift]...)
		}
	case bmsAttachmentOrderShuffle:
		p.rng.Shuffle(n, func(i, j int) { out[i], out[j] = out[j], out[i] })
	}
	names := make([]string, 0, n)
	for _, it := range out {
		if m, ok := it.(map[string]interface{}); ok {
			if s, ok := m["name"].(string); ok {
				names = append(names, s)
			}
		}
	}
	p.orders = append(p.orders, names)
	return out
}

func (p *bmsAttachmentOrderProxy) modifyResponse(resp *http.Response) error {
	if resp.Request == nil || resp.Request.Method != http.MethodGet || resp.StatusCode != http.StatusOK {
		return nil
	}
	if !bmsGetPathRe.MatchString(resp.Request.URL.Path) {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(body, &doc); err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}
	if items, ok := doc["network_attachments"].([]interface{}); ok && len(items) > 1 {
		doc["network_attachments"] = p.reorder(items)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	resp.Body = io.NopCloser(bytes.NewReader(out))
	resp.ContentLength = int64(len(out))
	resp.Header.Set("Content-Length", fmt.Sprint(len(out)))
	resp.Header.Del("Content-Encoding")
	return nil
}

// newBmsAttachmentOrderProxy starts the proxy in front of the VPC endpoint in
// IBMCLOUD_IS_NG_API_ENDPOINT and points the provider at the proxy for the
// rest of the test. It returns nil (and skips the test) when no endpoint is
// configured, because the proxy needs an explicit endpoint to forward to.
func newBmsAttachmentOrderProxy(t *testing.T) *bmsAttachmentOrderProxy {
	t.Helper()
	upstreamRaw := os.Getenv("IBMCLOUD_IS_NG_API_ENDPOINT")
	if upstreamRaw == "" {
		t.Skip("IBMCLOUD_IS_NG_API_ENDPOINT must point at a VPC API endpoint (for example the ibm-mock-servers vpc-latest mock) for this test")
	}
	upstream, err := url.Parse(upstreamRaw)
	if err != nil {
		t.Fatalf("IBMCLOUD_IS_NG_API_ENDPOINT is not a URL: %s", err)
	}
	basePath := strings.TrimSuffix(upstream.Path, "/")
	host := upstream.Host
	upstream.Path = ""
	upstream.RawPath = ""

	p := &bmsAttachmentOrderProxy{
		mode: bmsAttachmentOrderSame,
		rng:  rand.New(rand.NewSource(20250101)),
	}
	rp := httputil.NewSingleHostReverseProxy(upstream)
	director := rp.Director
	rp.Director = func(req *http.Request) {
		director(req)
		// The mocks validate the Host header, send the upstream host name.
		req.Host = host
		// The provider is sent the proxy URL with the same base path, so the
		// path is forwarded unchanged; just make sure the base path is there.
		if basePath != "" && !strings.HasPrefix(req.URL.Path, basePath) {
			req.URL.Path = basePath + req.URL.Path
		}
		// Ask for an uncompressed body so the response can be rewritten.
		req.Header.Del("Accept-Encoding")
	}
	rp.ModifyResponse = p.modifyResponse
	p.server = httptest.NewServer(rp)
	t.Cleanup(p.server.Close)

	t.Setenv("IBMCLOUD_IS_NG_API_ENDPOINT", p.server.URL+basePath)
	return p
}

// The three attachment layouts the tests create: seven secondary attachments
// named pci2 to pci8 (or vlan2 to vlan8) in that order, one VNI each.
type bmsAttachmentLayout struct {
	name   string
	config func(vpcname, subnetname, sshname, publicKey, bmsname string) string
	// names of the seven secondary attachments in config order
	names func(bmsname string) []string
}

func bmsSevenNames(bmsname string, kinds []string) []string {
	out := make([]string, 0, len(kinds))
	for i, k := range kinds {
		out = append(out, fmt.Sprintf("%s-%s%d", bmsname, k, i+2))
	}
	return out
}

var bmsAttachmentLayouts = []bmsAttachmentLayout{
	{
		name:   "pci",
		config: testAccCheckIBMISBareMetalServerSevenPciAttachmentsConfig,
		names: func(bmsname string) []string {
			return bmsSevenNames(bmsname, []string{"pci", "pci", "pci", "pci", "pci", "pci", "pci"})
		},
	},
	{
		name:   "pci-vlan-mix",
		config: testAccCheckIBMISBareMetalServerPciVlanMixAttachmentsConfig,
		names: func(bmsname string) []string {
			return bmsSevenNames(bmsname, []string{"pci", "pci", "pci", "vlan", "vlan", "vlan", "vlan"})
		},
	},
	{
		name:   "vlan",
		config: testAccCheckIBMISBareMetalServerSevenVlanAttachmentsConfig,
		names: func(bmsname string) []string {
			return bmsSevenNames(bmsname, []string{"vlan", "vlan", "vlan", "vlan", "vlan", "vlan", "vlan"})
		},
	},
}

// checkBmsAttachmentNamesInConfigOrder asserts network_attachments is stored in
// the order the configuration declares them, whatever order the API used.
func checkBmsAttachmentNamesInConfigOrder(resourceName string, names []string) resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(resourceName, "network_attachments.#", fmt.Sprint(len(names))),
	}
	for i, n := range names {
		checks = append(checks, resource.TestCheckResourceAttr(resourceName, fmt.Sprintf("network_attachments.%d.name", i), n))
	}
	return resource.ComposeAggregateTestCheckFunc(checks...)
}

// TestAccIBMISBareMetalServer_networkAttachmentsOrder_sameOrder is the control
// case: the API returns the attachments in the order they were created, so
// there is no drift. It passes with or without the fix.
func TestAccIBMISBareMetalServer_networkAttachmentsOrder_sameOrder(t *testing.T) {
	for _, layout := range bmsAttachmentLayouts {
		layout := layout
		t.Run(layout.name, func(t *testing.T) {
			runBmsAttachmentOrderTest(t, layout, []string{bmsAttachmentOrderSame, bmsAttachmentOrderSame})
		})
	}
}

// TestAccIBMISBareMetalServer_networkAttachmentsOrder_differentOrder is the
// reported bug: the server is created with the attachments in config order,
// then every later GET returns them reversed, rotated or shuffled. Terraform
// must still see no change, because nothing on the server changed.
func TestAccIBMISBareMetalServer_networkAttachmentsOrder_differentOrder(t *testing.T) {
	for _, layout := range bmsAttachmentLayouts {
		layout := layout
		t.Run(layout.name, func(t *testing.T) {
			runBmsAttachmentOrderTest(t, layout, []string{bmsAttachmentOrderReverse, bmsAttachmentOrderShuffle, bmsAttachmentOrderRotate})
		})
	}
}

// TestAccIBMISBareMetalServer_networkAttachmentsOrder_emptyAllowedVlans is the
// second row from the report: PCI attachments declared without allowed_vlans
// come back from the API with allowed_vlans [], and the provider stores that
// empty set, so the next plan shows "+ allowed_vlans = []" on every row. The
// API answers in creation order here, so any plan is from that row alone.
func TestAccIBMISBareMetalServer_networkAttachmentsOrder_emptyAllowedVlans(t *testing.T) {
	layout := bmsAttachmentLayout{
		name:   "pci-no-vlans",
		config: testAccCheckIBMISBareMetalServerSevenPciAttachmentsNoAllowedVlansConfig,
		names: func(bmsname string) []string {
			return bmsSevenNames(bmsname, []string{"pci", "pci", "pci", "pci", "pci", "pci", "pci"})
		},
	}
	runBmsAttachmentOrderTest(t, layout, []string{bmsAttachmentOrderSame})
}

// runBmsAttachmentOrderTest creates the server once (the API answers in
// creation order during create), then runs one plan-only step per order in
// readOrders. Each of those steps must produce an empty plan and keep the
// attachments in state in config order.
func runBmsAttachmentOrderTest(t *testing.T, layout bmsAttachmentLayout, readOrders []string) {
	proxy := newBmsAttachmentOrderProxy(t)
	resourceName := "ibm_is_bare_metal_server.testacc_bms"
	suffix := acctest.RandIntRange(10, 100)
	vpcname := fmt.Sprintf("tf-vpc-%s-%d", layout.name, suffix)
	subnetname := fmt.Sprintf("tf-subnet-%s-%d", layout.name, suffix)
	sshname := fmt.Sprintf("tf-sshkey-%s-%d", layout.name, suffix)
	bmsname := fmt.Sprintf("tf-bms-%s-%d", layout.name, suffix)
	publicKey := strings.TrimSpace(`
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCKVmnMOlHKcZK8tpt3MP1lqOLAcqcJzhsvJcjscgVERRN7/9484SOBJ3HSKxxNG5JN8owAjy5f9yYwcUg+JaUVuytn5Pv3aeYROHGGg+5G346xaq3DAwX6Y5ykr2fvjObgncQBnuU5KHWCECO/4h8uWuwh/kfniXPVjFToc+gnkqA+3RKpAecZhFXwfalQ9mMuYGFxn+fwn8cYEApsJbsEmb0iJwPiZ5hjFC8wREuiTlhPHDgkBLOiycd20op2nXzDbHfCHInquEe/gYxEitALONxm0swBOwJZwlTDOB7C6y2dzlrtxr1L59m7pCkWI4EtTRLvleehBoj3u7jB4usR
`)
	config := layout.config(vpcname, subnetname, sshname, publicKey, bmsname)
	names := layout.names(bmsname)

	steps := []resource.TestStep{
		{
			PreConfig: func() { proxy.setMode(bmsAttachmentOrderSame) },
			Config:    config,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(resourceName, "name", bmsname),
				checkBmsAttachmentNamesInConfigOrder(resourceName, names),
			),
		},
	}
	for _, order := range readOrders {
		order := order
		steps = append(steps, resource.TestStep{
			PreConfig: func() { proxy.setMode(order) },
			Config:    config,
			PlanOnly:  true,
			// A plan-only step fails when the plan is not empty. With the bug,
			// the refresh writes the shuffled list to state and the plan shows
			// every row as changed. Check functions do not run on plan-only
			// steps, so the order the API answered with is verified below.
		})
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { acc.TestAccPreCheck(t) },
		ProviderFactories: acc.TestAccProviderFactories(),
		CheckDestroy:      testAccCheckIBMISBareMetalServerDestroy,
		Steps:             steps,
	})

	// Make sure the proxy really served the attachments out of config order,
	// otherwise an empty plan proves nothing. The primary attachment is part
	// of the API list, so compare the secondary names only.
	wantShuffled := false
	for _, order := range readOrders {
		if order != bmsAttachmentOrderSame {
			wantShuffled = true
		}
	}
	seenShuffled := false
	for _, got := range proxy.allOrders() {
		secondaries := make([]string, 0, len(got))
		for _, n := range got {
			if n != bmsname+"-pci1" {
				secondaries = append(secondaries, n)
			}
		}
		if len(secondaries) == len(names) && !reflect.DeepEqual(secondaries, names) {
			seenShuffled = true
			t.Logf("API answered with attachments out of config order: %v", secondaries)
			break
		}
	}
	if wantShuffled && !seenShuffled {
		t.Fatalf("the proxy never served the attachments out of config order; recorded orders: %v", proxy.allOrders())
	}
}

func testAccCheckIBMISBareMetalServerAttachmentsPreamble(vpcname, subnetname, sshname, publicKey string) string {
	return fmt.Sprintf(`
		resource "ibm_is_vpc" "testacc_vpc" {
			name = "%s"
		}

		resource "ibm_is_subnet" "testacc_subnet" {
			name                     = "%s"
			vpc                      = ibm_is_vpc.testacc_vpc.id
			zone                     = "%s"
			total_ipv4_address_count = 64
		}

		resource "ibm_is_ssh_key" "testacc_sshkey" {
			name       = "%s"
			public_key = "%s"
		}
`, vpcname, subnetname, acc.ISZoneName, sshname, publicKey)
}

func testAccCheckIBMISBareMetalServerAttachmentsHeader(bmsname string) string {
	return fmt.Sprintf(`
		resource "ibm_is_bare_metal_server" "testacc_bms" {
			profile = "%s"
			name    = "%s"
			image   = "%s"
			zone    = "%s"
			keys    = [ibm_is_ssh_key.testacc_sshkey.id]
			vpc     = ibm_is_vpc.testacc_vpc.id
			primary_network_attachment {
				name = "%s-pci1"
				virtual_network_interface {
					name                      = "%s-pci1-vni"
					subnet                    = ibm_is_subnet.testacc_subnet.id
					enable_infrastructure_nat = true
					allow_ip_spoofing         = true
				}
				allowed_vlans = [100, 101, 102, 103, 104, 105, 106]
			}
`, acc.IsBareMetalServerProfileName, bmsname, acc.IsBareMetalServerImage, acc.ISZoneName, bmsname, bmsname)
}

// bmsPciAttachmentBlock renders one secondary PCI attachment. allowedVlans is
// the HCL for allowed_vlans, or "" to send nothing (the API then returns []).
func bmsPciAttachmentBlock(bmsname string, idx int, allowedVlans string) string {
	vlans := ""
	if allowedVlans != "" {
		vlans = "\t\t\t\tallowed_vlans = " + allowedVlans + "\n"
	}
	return fmt.Sprintf(`
			network_attachments {
				name = "%[1]s-pci%[2]d"
				virtual_network_interface {
					name                      = "%[1]s-pci%[2]d-vni"
					subnet                    = ibm_is_subnet.testacc_subnet.id
					enable_infrastructure_nat = true
				}
%[3]s			}
`, bmsname, idx, vlans)
}

// bmsPciAllowedVlans gives each secondary PCI attachment its own VLAN range,
// so the allowed_vlans of two PCI attachments never overlap.
func bmsPciAllowedVlans(idx int) string {
	return fmt.Sprintf("[%d, %d]", 200+idx*10, 201+idx*10)
}

func bmsVlanAttachmentBlock(bmsname string, idx int, vlan int) string {
	return fmt.Sprintf(`
			network_attachments {
				name = "%[1]s-vlan%[2]d"
				vlan = %[3]d
				virtual_network_interface {
					name                      = "%[1]s-vlan%[2]d-vni"
					subnet                    = ibm_is_subnet.testacc_subnet.id
					enable_infrastructure_nat = true
				}
			}
`, bmsname, idx, vlan)
}

// Seven secondary PCI attachments, pci2 to pci8, one VNI each.
func testAccCheckIBMISBareMetalServerSevenPciAttachmentsConfig(vpcname, subnetname, sshname, publicKey, bmsname string) string {
	var b strings.Builder
	b.WriteString(testAccCheckIBMISBareMetalServerAttachmentsPreamble(vpcname, subnetname, sshname, publicKey))
	b.WriteString(testAccCheckIBMISBareMetalServerAttachmentsHeader(bmsname))
	for i := 2; i <= 8; i++ {
		b.WriteString(bmsPciAttachmentBlock(bmsname, i, bmsPciAllowedVlans(i)))
	}
	b.WriteString("\t\t}\n")
	return b.String()
}

// Seven secondary PCI attachments without allowed_vlans, the shape from the
// report: the API answers with allowed_vlans [] for each of them.
func testAccCheckIBMISBareMetalServerSevenPciAttachmentsNoAllowedVlansConfig(vpcname, subnetname, sshname, publicKey, bmsname string) string {
	var b strings.Builder
	b.WriteString(testAccCheckIBMISBareMetalServerAttachmentsPreamble(vpcname, subnetname, sshname, publicKey))
	b.WriteString(testAccCheckIBMISBareMetalServerAttachmentsHeader(bmsname))
	for i := 2; i <= 8; i++ {
		b.WriteString(bmsPciAttachmentBlock(bmsname, i, ""))
	}
	b.WriteString("\t\t}\n")
	return b.String()
}

// Three secondary PCI attachments (pci2 to pci4) and four VLAN attachments
// (vlan5 to vlan8) on VLAN IDs the primary PCI attachment allows.
func testAccCheckIBMISBareMetalServerPciVlanMixAttachmentsConfig(vpcname, subnetname, sshname, publicKey, bmsname string) string {
	var b strings.Builder
	b.WriteString(testAccCheckIBMISBareMetalServerAttachmentsPreamble(vpcname, subnetname, sshname, publicKey))
	b.WriteString(testAccCheckIBMISBareMetalServerAttachmentsHeader(bmsname))
	for i := 2; i <= 4; i++ {
		b.WriteString(bmsPciAttachmentBlock(bmsname, i, bmsPciAllowedVlans(i)))
	}
	for i := 5; i <= 8; i++ {
		b.WriteString(bmsVlanAttachmentBlock(bmsname, i, 100+i-5))
	}
	b.WriteString("\t\t}\n")
	return b.String()
}

// Seven secondary VLAN attachments, vlan2 to vlan8, on VLAN IDs the primary
// PCI attachment allows.
func testAccCheckIBMISBareMetalServerSevenVlanAttachmentsConfig(vpcname, subnetname, sshname, publicKey, bmsname string) string {
	var b strings.Builder
	b.WriteString(testAccCheckIBMISBareMetalServerAttachmentsPreamble(vpcname, subnetname, sshname, publicKey))
	b.WriteString(testAccCheckIBMISBareMetalServerAttachmentsHeader(bmsname))
	for i := 2; i <= 8; i++ {
		b.WriteString(bmsVlanAttachmentBlock(bmsname, i, 100+i-2))
	}
	b.WriteString("\t\t}\n")
	return b.String()
}
