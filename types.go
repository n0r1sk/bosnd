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
	v *bool
	t *bool
}

type rc struct {
	config *Config
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
	Cmd            struct {
		Start       []string
		Reload      []string
		Processname string
	}
	Pdns struct {
		Apiurl       string // this is the enable Pdns config switch
		Apikey       string
		Ipaddress    string
		Domainprefix string
		Domainzone   string
	}
	Templates map[string]Configfilepair
	Swarm     struct {
		Cacertpem     string
		Clientcertpem string
		Clientkeypem  string
		Domainprefix  string
		Domainzone    string
		Managerurl    string
		Usesocket     bool
		Services      *[]Service
		Networks      []string // this is the enable Swarm config switch
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
