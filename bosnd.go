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
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"syscall"
	"text/template"
	"time"

	"golang.org/x/net/context"

	_ "net/http/pprof"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/logrusorgru/aurora"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	log "github.com/sirupsen/logrus"
)

var mainloop bool
var ctrlcmd *exec.Cmd
var dockerclient *client.Client
var configfile string

var Version string
var Versionname string
var Build string
var Buildtime string

var configReloads = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "bosnd_service_configuration_reloads",
		Help: "The count of the configuration reloads since start.",
	},
)

func isprocessrunningps(config *Config) (running bool) {
	// get all folders from proc filesystem
	running = false

	files, _ := ioutil.ReadDir("/proc")
	for _, f := range files {

		// check if folder is a integer (process number)
		if _, err := strconv.Atoi(f.Name()); err == nil {
			// open status file of process
			f, err := os.Open("/proc/" + f.Name() + "/status")
			if err != nil {
				log.Info(err)
				return running
			}

			// read status line by line
			scanner := bufio.NewScanner(f)

			// check if process name in status of process

			for scanner.Scan() {

				re := regexp.MustCompile("^Name:\\s*" + config.Cmd.Processname + ".*")
				match := re.MatchString(scanner.Text())

				if match == true {
					running = true
					log.Debug("Process running: " + strconv.FormatBool(running))
				}

			}
			if running == true {
				return running
			}

		}

	}

	return running

}

var versionTemplate = `The Docker Bosnd - made by https://www.n0r1sk.com
-------------------------------------------------
Version:        {{.Version}}
Version Name:   {{.Versionname}}
Build:          {{.Build}}
Buildtime:      {{.Buildtime}}`

func startprocess(config *Config) {
	log.Info("Start Process!")
	cmd := exec.Command(config.Cmd.Start[0], config.Cmd.Start[1:]...)

	// Attach the process to OS stderr, OS stdout
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	// Start the process
	err := cmd.Start()

	if err != nil {
		log.Warn(aurora.Cyan(err.Error()))
	}
	ctrlcmd = cmd

	// just give the process some time to start
	time.Sleep(time.Duration(250) * time.Millisecond)
	ok := isprocessrunningps(config)
	if ok == true {
		log.Info(aurora.Green("Process started"))
	}
}

func reloadprocess(config *Config) {
	log.Info("Reloading Process!")
	cmd := exec.Command(config.Cmd.Reload[0], config.Cmd.Reload[1:]...)
	err := cmd.Start()
	if err != nil {
		log.Warn(aurora.Cyan(err.Error()))
	}
	cmd.Wait()
	isprocessrunningps(config)
}

func writeconfig(config *Config) (changed bool) {

	changed = false

	log.Debug(config.Templates)
	for k, v := range config.Templates {
		log.Debug("Processing Template: " + k)
		log.Debug("Template values: ", v)

		//  open template
		t, err := template.ParseFiles(v.Src)
		if err != nil {
			log.Error(err)
			continue
		}

		// process template
		var tpl bytes.Buffer
		err = t.Execute(&tpl, config.Swarm)
		if err != nil {
			log.Error(err)
			continue
		}

		// create md5 of result
		md5tpl := fmt.Sprintf("%x", md5.Sum([]byte(tpl.String())))
		log.Debug("MD5 of CONF " + v.Src + ": " + md5tpl)
		// log.Debug("TPL: " + tpl.String())

		// open existing config, read it to memory
		exconf, errexconf := ioutil.ReadFile(v.Dst)
		if errexconf != nil {
			log.Warn("Cannot read existing conf!")
			log.Warn(errexconf)
		}

		md5exconf := fmt.Sprintf("%x", md5.Sum(exconf))
		log.Debug("MD5 of EXCONF" + v.Src + ": " + md5exconf)
		// log.Debug("TPL: " + string(exconf[:]))

		// comapre md5 and write config if needed
		if md5tpl == md5exconf {
			log.Info(aurora.Green("MD5 sums of " + v.Src + " equal! Nothing to do."))
			continue
		}

		log.Info(aurora.Brown("MD5 sums of  " + v.Src + " different writing new conf!"))

		// overwrite existing conf
		err = ioutil.WriteFile(v.Dst, []byte(tpl.String()), 0644)
		if err != nil {
			log.Error("Cannot write config file.")
			log.Error(err)
		}

		changed = true

	}

	return changed

}

func getsericelabel(ctx context.Context, servicename string) (map[string]string, error) {
	f := filters.NewArgs()
	f.Add("name", servicename)

	opts := types.ServiceListOptions{
		Filters: f,
	}
	s, _ := dockerclient.ServiceList(ctx, opts)

	labels := s[0].Spec.Labels

	if len(labels) == 0 {
		return labels, errors.New("Service " + servicename + " has no context label!")
	}

	return labels, nil
}

func getorrefreshdockerclient(config *Config) bool {

	ok := false

	for ok == false {

		if config.Swarm.Usesocket {
			var err error
			dockerclient, err = client.NewClient("unix:///var/run/docker.sock", "", nil, nil)
			if err != nil {
				log.Warn(err)
				time.Sleep(time.Duration(config.Checkintervall) * time.Second)
				continue
			}
		} else {
			cert, err := tls.LoadX509KeyPair(config.Swarm.Clientcertpem, config.Swarm.Clientkeypem)
			if err != nil {
				log.Warn(err)
				time.Sleep(time.Duration(config.Checkintervall) * time.Second)
				continue
			}

			caCert, err := ioutil.ReadFile(config.Swarm.Cacertpem)
			if err != nil {
				log.Warn(err)
				time.Sleep(time.Duration(config.Checkintervall) * time.Second)
				continue
			}

			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)

			tlsConfig := &tls.Config{
				Certificates:       []tls.Certificate{cert},
				RootCAs:            caCertPool,
				InsecureSkipVerify: true,
			}

			tlsConfig.BuildNameToCertificate()
			timeout := time.Duration(5 * time.Second)

			transport := &http.Transport{TLSClientConfig: tlsConfig}
			httpclient := &http.Client{Transport: transport, Timeout: timeout}

			dockerclient, err = client.NewClient(config.Swarm.Managerurl, "", httpclient, nil)

			if err != nil {
				log.Warn(err)
				time.Sleep(time.Duration(config.Checkintervall) * time.Second)
				continue
			}
		}
		ok = true
	}

	return ok
}

func getservicesofnet(config *Config) error {

	ctx := context.Background()
	version, err := dockerclient.ServerVersion(ctx)

	if err != nil {
		getorrefreshdockerclient(config)
		return err
	}
	log.Debug(version)

	swarmservices := Swarmservices{}

	for _, netwn := range config.Swarm.Networks {

		f := filters.NewArgs()
		f.Add("name", netwn)

		opts := types.NetworkListOptions{
			Filters: f,
		}

		nl, err := dockerclient.NetworkList(ctx, opts)

		if err != nil {
			getorrefreshdockerclient(config)
			return err
		}

		if len(nl) == 0 {
			log.Warn("Given network not found: " + netwn)
			continue
		}

		log.Debug(nl[0].ID)

		n, err := dockerclient.NetworkInspect(ctx, nl[0].ID, types.NetworkInspectOptions{Verbose: true})

		if err != nil {
			return err
		}

		for k, s := range n.Services {
			if k == "" {
				continue
			}

			ms := Service{}
			ms.Name = k

			ms.Labels, err = getsericelabel(ctx, ms.Name)

			if err != nil {
				return err
			}

			// get ips to sclice
			var tmpips []string
			for _, t := range s.Tasks {
				tmpips = append(tmpips, t.EndpointIP)
			}

			sort.Strings(tmpips)

			cnt := 1
			for _, ip := range tmpips {
				me := Endpoint{}
				me.Address = ip
				me.Hostname = k + "-" + strconv.Itoa(cnt)
				ms.Endpoints = append(ms.Endpoints, me)
				cnt++
			}

			swarmservices.Services = append(swarmservices.Services, ms)
		}

	}

	// sort the struct to get deterministic sequence, first the services by name
	sort.Slice(swarmservices.Services, func(i, j int) bool {
		return swarmservices.Services[i].Name < swarmservices.Services[j].Name
	})

	// second, inside the services, by endpoint hostnames
	for _, srv := range swarmservices.Services {
		sort.Slice(srv.Endpoints, func(i, j int) bool {
			return srv.Endpoints[i].Hostname > srv.Endpoints[j].Hostname
		})
	}

	config.Swarm.Services = &swarmservices.Services

	return err

	//j, _ := json.Marshal(*config.Swarm.Services)
	//log.Debug(string(j[:]))
}

func parsecmdline() *flgs {

	f := flgs{}

	f.c = flag.String("c", "/config/bosnd.yml", "config file including path")
	f.v = flag.Bool("v", false, "Print the version and exit")

	flag.Parse()

	return &f
}

func prom(config *Config) {
	prometheus.MustRegister(configReloads)

	flag.Parse()
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe("0.0.0.0:"+config.Prometheus.Port, nil))
}

func (rcontrol *rc) reload(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	if params["id"] == rcontrol.config.Control.Key {
		log.Info("Reload triggered!")
		reloadprocess(rcontrol.config)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Reloaded!"))
	}
}

func api(config *Config) {
	router := mux.NewRouter()
	r := &rc{config: config}
	router.HandleFunc("/reload/{id}", r.reload).Methods("GET")
	log.Fatal(http.ListenAndServe("0.0.0.0:"+config.Control.Port, router))
}

func main() {

	// ignore all signals of child, the kernel will clean them up, no zombies
	signal.Ignore(syscall.SIGCHLD)

	// configure logrus logger
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	customFormatter.ForceColors = true
	log.SetFormatter(customFormatter)
	log.SetOutput(os.Stdout)

	fl := parsecmdline()

	if *fl.v == true {

		type v struct {
			Version     string
			Versionname string
			Build       string
			Buildtime   string
		}

		actversion := v{}
		actversion.Build = Build

		if Version == "" {
			actversion.Version = "Manual Build!"
		} else {
			actversion.Version = Version
		}

		if Buildtime == "" {
			actversion.Buildtime = time.Now().String()
		} else {
			actversion.Buildtime = Buildtime
		}

		actversion.Versionname = Versionname

		tmpl, err := template.New("").Parse(versionTemplate)
		if err != nil {
			log.Warn(err)
		}

		var tpl bytes.Buffer
		tmpl.Execute(&tpl, actversion)
		if err != nil {
			log.Warn(err)
		}

		fmt.Print(tpl.String() + "\n")

		os.Exit(0)
	}

	configfile = *fl.c
	config, ok := ReadConfigfile(configfile)
	if !ok {
		log.Warn(aurora.Red("Error during config parsing, yet continuing!"))
	}

	// set check intervall from config
	if config.Checkintervall == 0 {
		config.Checkintervall = 30
	}

	// get debug flag from config
	if config.Debug == true {
		log.SetLevel(log.DebugLevel)
		go func() {
			log.Println(http.ListenAndServe("0.0.0.0:"+config.Debugport, nil))
		}()
	}

	log.Debug("Configfile Keys and Values: ", config)

	// check if PDNS configuration is enabled
	if config.Pdns.Apikey != "" {
		updatepdns(*config)
	}

	// check if Prometheus is enabled
	if config.Prometheus.Port != "" {
		go prom(config)
	}

	// only take the swarm into accout if it is configured
	if len(config.Swarm.Networks) != 0 {
		// get docker client for swarm
		ok = getorrefreshdockerclient(config)
		if ok != true {
			log.Debug("Unable to create Docker Api client!")
		}
	}

	// only take the control into accout if it is configured
	if config.Control.Key != "" {
		go api(config)
	}

	// this will loop forever
	mainloop = true
	var changed = false

	for mainloop == true {
		// reread config file
		ok := ReReadConfigfile(configfile, config)
		if !ok {
			log.Warn(aurora.Red("Error during config parsing, yet continuing!"))
			time.Sleep(time.Duration(config.Checkintervall) * time.Second)
			continue
		}

		if len(config.Swarm.Networks) != 0 {
			// get services from Docker network and Docker services
			// work with the local working config inside the loop
			err := getservicesofnet(config)
			if err != nil {
				log.Debug(err)
				log.Warn(aurora.Red("Error during retrieving information: " + err.Error()))
				time.Sleep(time.Duration(config.Checkintervall) * time.Second)
				continue
			}
		}

		// process config
		changed = writeconfig(config)

		if changed == true {
			if isprocessrunningps(config) {
				reloadprocess(config)
				configReloads.Inc()
			} else {
				startprocess(config)
				configReloads.Inc()
			}
		} else {
			if !isprocessrunningps(config) {
				startprocess(config)
			}
		}

		time.Sleep(time.Duration(config.Checkintervall) * time.Second)
	}
}
