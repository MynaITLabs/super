/*
Routines for managing the uplink interfaces, for outbound internet
*/
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
)

/* WPA Supplicant Support */

var WpaConfigPath = TEST_PREFIX + "/configs/wifi_uplink/wpa.json"

var WPAmtx sync.Mutex

type WPANetwork struct {
	Disabled bool
	Password string
	SSID     string
	KeyMgmt  string
	Priority string `json:",omitempty"`
	BSSID    string `json:",omitempty"`
}

type WPAIface struct {
	Iface    string
	Enabled  bool
	Networks []WPANetwork
}

type WPASupplicantConfig struct {
	WPAs []WPAIface
}

func (n *WPANetwork) Validate() error {
	// Check for newlines in Password field
	if strings.Contains(n.Password, "\n") {
		return fmt.Errorf("Password field contains newline characters")
	}

	// Check for newlines in SSID field
	if strings.Contains(n.SSID, "\n") {
		return fmt.Errorf("SSID field contains newline characters")
	}

	if n.Priority != "" {
		_, err := strconv.Atoi(n.Priority)
		if err != nil {
			return fmt.Errorf("Priority field must contain numeric value")
		}
	}

	if n.BSSID != "" {
		// Check if BSSID field is a valid MAC address
		match, err := regexp.MatchString("^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$", n.BSSID)
		if err != nil || !match {
			return fmt.Errorf("BSSID field must be a valid MAC address")
		}
	}

	if n.KeyMgmt == "" {
		return fmt.Errorf("KeyMgmt field must be set (WPA-PSK WPA-PSK-SHA256 or WPA-PSK WPA-PSK-SHA256 SAE)")
	}

	parts := strings.Split(n.KeyMgmt, " ")
	for _, part := range parts {
		if part == "WPA-PSK" {
			continue
		} else if part == "WPA-PSK-SHA256" {
			continue
		} else if part == "SAE" {
			continue
		}
		return fmt.Errorf("KeyMgmt field has invalid field " + part)
	}

	return nil
}

func isWifiUplinkIfaceEnabled(Name string, interfaces []InterfaceConfig) bool {
	for _, iface := range interfaces {
		if iface.Name == Name {
			if iface.Type == "Uplink" && iface.Subtype == "wifi" {
				return iface.Enabled
			}
			break
		}
	}
	return false
}

func writeWPAs(interfaces []InterfaceConfig, config WPASupplicantConfig) error {
	//assumes lock is held

	for _, wpa := range config.WPAs {

		//only keep an iface for the config if enabled
		//and the subtype matches
		if !wpa.Enabled {
			continue
		}

		tmpl, err := template.New("wpa_supplicant.conf").Parse(`# Note this is an autogenerated file
      ctrl_interface=DIR=/var/run/wpa_supplicant_` + wpa.Iface + `
      {{range .Networks}}
      {{if not .Disabled}}
      network={
      	ssid="{{.SSID}}"
      	psk="{{.Password}}"
      	{{if .Priority}}priority={{.Priority}}{{end}}
      	{{if .BSSID}}bssid={{.BSSID}}{{end}}
        key_mgmt={{.KeyMgmt}}
      }
      {{end}}
      {{end}}`)

		if err != nil {
			log.Println("Error parsing template:", err)
			return err
		}

		var result bytes.Buffer
		err = tmpl.Execute(&result, wpa)
		if err != nil {
			log.Println("Error executing template:", err)
			return err
		}
		fp := TEST_PREFIX + "/configs/wifi_uplink/wpa_" + wpa.Iface + ".conf"
		err = ioutil.WriteFile(fp, result.Bytes(), 0600)
		if err != nil {
			return err
		}
	}

	return nil
}

func loadWpaConfig() (WPASupplicantConfig, error) {
	WPAmtx.Lock()
	defer WPAmtx.Unlock()

	return loadWpaConfigLocked()
}
func loadWpaConfigLocked() (WPASupplicantConfig, error) {
	config := WPASupplicantConfig{}

	data, err := ioutil.ReadFile(WpaConfigPath)
	if err != nil {
		log.Println(err)
		return config, err
	} else {
		err = json.Unmarshal(data, &config)
		if err != nil {
			log.Println(err)
			return config, err
		}
	}
	return config, nil
}

func insertWpaConfigAndSave(interfaces []InterfaceConfig, new_wpa WPAIface) error {
	//assumes new_wpa is validated or empty and ignored
	// will clear out any wpas that are *not* set to uplink & wifi
	// in interfaces anymore
	// as well as inserting new_wpa
	WPAmtx.Lock()
	defer WPAmtx.Unlock()

	config := WPASupplicantConfig{}

	loaded, err := loadWpaConfigLocked()
	if err == nil {
		config = loaded
	}

	wpas := []WPAIface{}

	found := false
	for _, wpa := range config.WPAs {
		if wpa.Iface == new_wpa.Iface {
			wpas = append(wpas, new_wpa)
			found = true
			break
		} else {
			//update the enabled status
			wpa.Enabled = isWifiUplinkIfaceEnabled(wpa.Iface, interfaces)
			wpas = append(wpas, wpa)
		}
	}

	if !found && new_wpa.Iface != "" {
		wpas = append(wpas, new_wpa)
	}

	config.WPAs = wpas

	file, _ := json.MarshalIndent(config, "", " ")
	err = ioutil.WriteFile(WpaConfigPath, file, 0600)
	if err != nil {
		log.Println(err)
		return err
	}

	return writeWPAs(interfaces, config)
}

func getWpaSupplicantConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	config, err := loadWpaConfig()
	if err != nil {
		http.Error(w, "Failed to load wpa configuration", 400)
		return
	}
	json.NewEncoder(w).Encode(config)
}

func updateWpaSupplicantConfig(w http.ResponseWriter, r *http.Request) {
	wpa := WPAIface{}
	err := json.NewDecoder(r.Body).Decode(&wpa)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	enabled := false

	pattern := `^[a-zA-Z0-9]*(\.[a-zA-Z0-9]*)*$`
	matched, err := regexp.MatchString(pattern, wpa.Iface)
	if err != nil || !matched {
		log.Println("Invalid iface name", err)
		http.Error(w, "Invalid iface name", 400)
		return
	}

	for _, network := range wpa.Networks {

		//track whether at least one network is enabled.
		if !enabled {
			if network.Disabled == false {
				enabled = true
			}
		}

		if network.KeyMgmt == "" {
			network.KeyMgmt = "WPA-PSK WPA-PSK-SHA256"
		}
		err := network.Validate()
		if err != nil {
			log.Println("Validation error:", err)
			http.Error(w, "Failed to validate network "+err.Error(), 400)
			return
		}
	}

	//update the interface type
	interfaces, err := updateInterfaceType(wpa.Iface, "Uplink", "wifi", wpa.Enabled)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 400)
		return
	}

	err = insertWpaConfigAndSave(interfaces, wpa)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 400)
		return
	}

	uplink_plugin := "WIFI-UPLINK"

	started := false
	if enabled {
		// at least one network is on, so ensure that the plugin is on
		started = enablePlugin(uplink_plugin)
	}

	//even if all were disabled, make sure to restart to reflect that.
	if !started {
		//restart the service if
		restartPlugin(uplink_plugin)
	}

}

/* PPP Support */

// /configs/ppp is mounted to /etc
var PPPConfigPath = TEST_PREFIX + "/configs/ppp/ppp.json"

var PPPmtx sync.Mutex

type PPPIface struct {
	Iface    string
	Enabled  bool
	Username string
	Secret   string
	VLAN     string `json,optional`
	MTU      string `json, optional`
}

func (p *PPPIface) Validate() error {
	if p.Iface == "" {
		return fmt.Errorf("Iface field empty")
	}

	if p.Username == "" {
		return fmt.Errorf("Username field empty")
	}

	if strings.Contains(p.Username, "\n") {
		return fmt.Errorf("Username field contains newline characters")
	}

	if strings.Contains(p.Secret, "\n") {
		return fmt.Errorf("Secret field contains newline characters")
	}

	if p.VLAN != "" {
		_, err := strconv.Atoi(p.VLAN)
		if err != nil {
			return fmt.Errorf("VLAN field must contain numeric value")
		}
	}

	if p.MTU != "" {
		v, err := strconv.Atoi(p.MTU)
		if err != nil || v < 0 {
			return fmt.Errorf("MTU field must contain numeric positive value")
		}
	}

	return nil
}

type PPPConfig struct {
	PPPs []PPPIface
}

func loadPPPConfig() (PPPConfig, error) {
	PPPmtx.Lock()
	defer PPPmtx.Unlock()

	return loadPPPConfigLocked()
}
func loadPPPConfigLocked() (PPPConfig, error) {
	config := PPPConfig{}

	data, err := ioutil.ReadFile(PPPConfigPath)
	if err != nil {
		log.Println(err)
		return config, err
	} else {
		err = json.Unmarshal(data, &config)
		if err != nil {
			log.Println(err)
			return config, err
		}
	}
	return config, nil

}

func writePPP(interfaces []InterfaceConfig, config PPPConfig) error {
	//assumes lock is held

	//chap secrets hosts all credentials
	tmpl, err := template.New("chap-secrets").Parse(`# Note this is an autogenerated file
    # Secrets for authentication using CHAP
    # client        server  secret                  IP addresses

    {{range .PPPs}}
      "{{.Username}}" * "{{.Secret}}"
    {{end}}
    `)

	if err != nil {
		log.Println("Error parsing template:", err)
		return err
	}

	var result bytes.Buffer
	err = tmpl.Execute(&result, config)
	if err != nil {
		log.Println("Error executing chap-secrets template:", err)
		return err
	}
	fp := TEST_PREFIX + "/etc/ppp/chap-secrets"
	err = ioutil.WriteFile(fp, result.Bytes(), 0600)
	if err != nil {
		return err
	}

	for _, ppp := range config.PPPs {

		tmpl, err := template.New("provider").Parse(`# Note this is an autogenerated file
      # Minimalistic default options file for DSL/PPPoE connections
      noipdefault
      defaultroute
      replacedefaultroute
      persist
      {{if .MTU}}mtu {{.MTU}}{{end}}
      plugin rp-pppoe.so {{.Iface}}{{if .VLAN}}.{{.VLAN}}{{end}}
      {{if .BSSID}}bssid={{.BSSID}}{{end}}
      plugin rp-pppoe.so {{.Iface}}.{{.VLAN}}
      user "{{.Username}}"
      `)

		if err != nil {
			log.Println("Error parsing template:", err)
			return err
		}

		var result bytes.Buffer
		err = tmpl.Execute(&result, ppp)
		if err != nil {
			log.Println("Error executing chap-secrets template:", err)
			return err
		}

		fp := TEST_PREFIX + "/etc/ppp/provider_" + ppp.Iface
		err = ioutil.WriteFile(fp, result.Bytes(), 0600)
		if err != nil {
			return err
		}

	}

	return nil
}

func isPPPUplinkIfaceEnabled(Name string, interfaces []InterfaceConfig) bool {
	for _, iface := range interfaces {
		if iface.Name == Name {
			if iface.Type == "Uplink" && iface.Subtype == "ppp" {
				return iface.Enabled
			}
			break
		}
	}
	return false
}

func insertPPPConfigAndSave(interfaces []InterfaceConfig, new_ppp PPPIface) error {
	PPPmtx.Lock()
	defer PPPmtx.Unlock()

	config := PPPConfig{}

	loaded, err := loadPPPConfigLocked()
	if err == nil {
		config = loaded
	}

	ppps := []PPPIface{}

	found := false
	for _, ppp := range config.PPPs {
		if ppp.Iface == new_ppp.Iface {
			ppps = append(ppps, new_ppp)
			found = true
			break
		} else {
			//update the enabled status
			ppp.Enabled = isPPPUplinkIfaceEnabled(ppp.Iface, interfaces)
			ppps = append(ppps, ppp)
		}
	}

	if !found && new_ppp.Iface != "" {
		ppps = append(ppps, new_ppp)
	}

	config.PPPs = ppps

	file, _ := json.MarshalIndent(config, "", " ")
	err = ioutil.WriteFile(PPPConfigPath, file, 0600)
	if err != nil {
		log.Println(err)
		return err
	}

	return writePPP(interfaces, config)
}

func getPPPConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	config, err := loadPPPConfig()
	if err != nil {
		http.Error(w, "Failed to load ppp configuration", 400)
		return
	}
	json.NewEncoder(w).Encode(config)
}

func updatePPPConfig(w http.ResponseWriter, r *http.Request) {
	ppp := PPPIface{}
	err := json.NewDecoder(r.Body).Decode(&ppp)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	err = ppp.Validate()
	if err != nil {
		log.Println("Validation error:", err)
		http.Error(w, "Failed to validate ppp "+err.Error(), 400)
		return
	}

	//update the interface type
	interfaces, err := updateInterfaceType(ppp.Iface, "Uplink", "ppp", ppp.Enabled)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 400)
		return
	}

	err = insertPPPConfigAndSave(interfaces, ppp)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 400)
		return
	}

	ppp_plugin := "PPP"
	started := enablePlugin(ppp_plugin)
	if !started {
		restartPlugin(ppp_plugin)
	}

}

/* Setting IP */

func updateIPConfig(w http.ResponseWriter, r *http.Request) {
	iconfig := InterfaceConfig{}
	err := json.NewDecoder(r.Body).Decode(&iconfig)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	if iconfig.DisableDHCP == true {
		//validate router and ip
	}

	if iconfig.VLAN != "" {
		//validate vlan tag
	}

	err = updateInterfaceIP(iconfig)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 400)
		return
	}
}
