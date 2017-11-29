/*
Copyright 2017 Mario Kleinsasser and Bernhard Rausch

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// Recordsets A map of Recordsets as described by PowerDNS
type Recordsets struct {
	Rrsets []*Recordset `json:"rrsets"`
}

// Recordset A Recordset as described by PowerDNS
type Recordset struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	TTL        int      `json:"ttl"`
	Changetype string   `json:"changetype"`
	Records    []Record `json:"records"`
}

// Record A Record as described by PowerDNS
type Record struct {
	Content  string `json:"content"`
	Disabled bool   `json:"disabled"`
}

func recordsetstojson(data Recordsets) []byte {

	// set time to live to 10 if not set
	for _, e := range data.Rrsets {
		p := &e.TTL
		if *p == 0 {
			*p = 10
		}
	}

	b, err := json.Marshal(data)
	if err != nil {

	}

	log.Debug(string(b[:]))
	return b

}

func recordsetsreplace(apiurl string, apikey string, domainzone string, data Recordsets) {
	s := recordsetstojson(data)

	log.Debug(string(s[:]))

	req, err := http.NewRequest("PATCH", apiurl+"/"+domainzone+".", bytes.NewBuffer(s))
	req.Header.Set("X-API-Key", apikey)

	if err != nil {
		log.Warn(err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Warn(err)
		return
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	log.Debug("response Body:", string(body))
	log.Info("PDNS Update response code: " + resp.Status)
}

func updatepdns(config Config) {
	var rrset Recordsets
	var rs Recordset
	var r Record

	r.Content = config.Pdns.Ipaddress
	r.Disabled = false

	rs.Name = config.Pdns.Domainprefix + "." + config.Pdns.Domainzone + "."
	rs.Changetype = "REPLACE"
	rs.Type = "A"
	rs.Records = append(rs.Records, r)

	rrset.Rrsets = append(rrset.Rrsets, &rs)

	recordsetsreplace(config.Pdns.Apiurl, config.Pdns.Apikey, config.Pdns.Domainzone, rrset)
}
