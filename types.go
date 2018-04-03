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

type flgs struct {
	c *string
	b *bool
}

type rc struct {
	config *Config
}

// Record is the A record
type Record struct {
	Host string `json:"host"`
	TTL  int    `json:"ttl"`
}

// Endpoint Defines an endpoint (Docker task container)
type Endpoint struct {
	Hostname string
	Address  string
}

// Service Defines a Docker service
type Service struct {
	Name      string
	Endpoints []Endpoint
	Labels    map[string]string
}

// Swarmservices A slices of Docker services
type Swarmservices struct {
	Services []Service
}

// Pod The Pod information
type Pod struct {
	Labels   map[string]string
	Hostname string
	Address  string
}

// Configfilepair A pair of template source and resulting destination configfile
type Configfilepair struct {
	Src string
	Dst string
}

// Config Defines the main config struct and also holds the data for the templating
type Config struct {
	Debug          bool
	Debugport      string
	Checkintervall int64
	Templates      map[string]Configfilepair
	Cmd            struct {
		Start       []string
		Reload      []string
		Processname string
	}
	Kubernetes struct {
		Kubeconfig    string // Path to the Kubeconfig
		Labelselector string // Optional selector
		Domainprefix  string
		Domainzone    string
		Namespace     string // Enable switch is the existence of the environmet variable MY_NAMESPACE
		Services      *map[string][]Pod
	}
	Coredns struct {
		Arecord string   // The A-record which is need to be set
		TTL     int      // The ttl for the dns record set, default 60 seconds
		Path    string   // The CoreDNS path
		Etcd    []string // ETCD endpoints used for Coredns, this is the enable switch
	}
	Prometheus struct {
		Port string // this is the enable Prometheus config switch
	}
	Control struct {
		Port string
		Key  string // this is the enable Control config switch
	}
	Cron struct {
		Crontab string // this is the enable Cron config switch
	}
}
