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
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"golang.org/x/net/context"

	_ "net/http/pprof"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/logrusorgru/aurora"
	"github.com/robfig/cron"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Azure/azure-sdk-for-go/services/dns/mgmt/2017-09-01/dns"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"

	log "github.com/sirupsen/logrus"
)

var mainloop bool
var ctrlcmd *exec.Cmd
var dockerclient *client.Client
var configfile string
var kubeclient *kubernetes.Clientset

// Version is the version number used by the make script
var Version string

// Versionname is the codename used by the make script
var Versionname string

// Build is the build hash used by the make script
var Build string

// Buildtime is the timestamp used by the make script
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

		if len(config.Swarm.Networks) != 0 {
			err = t.Execute(&tpl, config.Swarm)
			if err != nil {
				log.Error(err)
				continue
			}
		}

		if config.Kubernetes.Namespace != "" {
			err = t.Execute(&tpl, config.Kubernetes)
			if err != nil {
				log.Error(err)
				continue
			}
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
		log.Debug("MD5 of EXCONF" + v.Dst + ": " + md5exconf)
		// log.Debug("TPL: " + string(exconf[:]))

		// comapre md5 and write config if needed
		log.Debug("MD5: " + md5tpl + " " + md5exconf)
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

func getservicelabel(ctx context.Context, servicename string) (map[string]string, error) {
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

			ms.Labels, err = getservicelabel(ctx, ms.Name)

			if err != nil {
				return err
			}

			// get the ips and the task slot to slice
			type tmpdata struct {
				ip   string
				slot string
			}

			var tmpdatas []tmpdata

			// get ips to sclice
			for _, t := range s.Tasks {
				log.Debug(t.Name + " " + t.EndpointIP)
				var td tmpdata
				td.ip = t.EndpointIP
				td.slot = strings.Split(t.Name, ".")[1]
				tmpdatas = append(tmpdatas, td)
			}

			sort.Slice(tmpdatas, func(i, j int) bool {
				return tmpdatas[i].slot < tmpdatas[j].slot
			})

			cnt := 1
			for _, d := range tmpdatas {
				me := Endpoint{}
				me.Address = d.ip
				me.Hostname = k + "-" + d.slot
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

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func getkubernetesclient() bool {

	log.Debug("Testing Kubeclient")
	ok := false

	for ok == false {
		var err error

		var kubeconfig string
		if home := homeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}

		// use the current context in kubeconfig
		kconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err.Error())
		}

		// create the clientset
		kubeclient, err = kubernetes.NewForConfig(kconfig)
		if err != nil {
			panic(err.Error())
		}

		_, err = kubeclient.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			log.Error(err)
			time.Sleep(time.Duration(10 * time.Second))
		} else {
			ok = true
		}

	}

	return ok

}

func getkubernetespods(config *Config) error {

	ok := getkubernetesclient()
	if ok != true {
		return errors.New("Failed to create Kubernetes Client")
	}

	pods, err := kubeclient.CoreV1().Pods(config.Kubernetes.Namespace).List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

	var apps map[string][]Pod
	// init map
	apps = make(map[string][]Pod)

	for _, p := range pods.Items {

		if p.Labels["context"] == "" || p.Labels["app"] == "" {
			continue
		}

		// create pod
		var po Pod
		po.Labels = p.Labels
		po.Hostname = p.Name
		po.Address = p.Status.PodIP

		// add to pod to app
		apps[p.Labels["app"]] = append(apps[p.Labels["app"]], po)

		log.Debug(po.Hostname + " " + po.Address)
	}

	for _, pods := range apps {
		sort.Slice(pods, func(i, j int) bool {
			return pods[i].Hostname < pods[j].Hostname
		})
	}

	config.Kubernetes.Apps = &apps
	log.Debug(config.Kubernetes.Apps)

	return nil
}

func azuredns(config *Config) error {

	// check if all env variables are here

	azuresubscriptionid := os.Getenv("AZURE_SUBSCRIPTION_ID")
	azureclientid := os.Getenv("AZURE_CLIENT_ID")
	azureclientsecret := os.Getenv("AZURE_CLIENT_SECRET")
	azuretenantid := os.Getenv("AZURE_TENANT_ID")
	azureresourcegroup := os.Getenv("AZURE_RESOURCE_GROUP")
	mypodip := os.Getenv("MY_POD_IP")

	if azuresubscriptionid == "" ||
		azureclientid == "" ||
		azureclientsecret == "" ||
		azuretenantid == "" ||
		azureresourcegroup == "" ||
		mypodip == "" {
		return errors.New("Not all Azure env variables are set")
	}

	vdnsClient := dns.NewRecordSetsClient(azuresubscriptionid)
	authorizer, err := auth.NewAuthorizerFromEnvironment()

	if err == nil {
		vdnsClient.Authorizer = authorizer
	}

	res, err := vdnsClient.Get(context.Background(), azureresourcegroup, config.Kubernetes.Domainzone, config.Kubernetes.Domainprefix, dns.A)

	if err != nil {
		// if the StatusCode = 404 we can proceed, because we are writing a new record
		if strings.Contains(err.Error(), "404") {
			log.Warn("Not found A Rec: " + config.Kubernetes.Domainprefix + " in " + config.Kubernetes.Domainzone)
		} else {
			return err
		}
	} else {
		meta := *res.RecordSetProperties.Metadata
		for _, v := range *res.RecordSetProperties.ARecords {
			log.Info("Previous IP Address: " + *v.Ipv4Address + " Last change: " + *meta["lastchangehuman"])
		}
	}

	log.Info("Creating A Rec: " + config.Kubernetes.Domainprefix + " in " + config.Kubernetes.Domainzone)

	t := time.Now()
	meta := make(map[string]*string)
	lastchangeepoch := strconv.FormatInt(t.UnixNano()/int64(time.Millisecond), 10)
	lastchangehuman := t.Format("2006-01-02T15:04:05.999-07:00")
	meta["lastchangeepoch"] = &lastchangeepoch
	meta["lastchangehuman"] = &lastchangehuman
	meta["namespace"] = &config.Kubernetes.Namespace

	rs := dns.RecordSet{
		Name: to.StringPtr(config.Kubernetes.Domainprefix),
		RecordSetProperties: &dns.RecordSetProperties{
			Metadata: &meta,
			TTL:      to.Int64Ptr(60),
			//Metadata: &make(map["lastchange"]strconv.FormatInt(time.Now().UnixNano() / int64(time.Millisecond, 10)))
			ARecords: &[]dns.ARecord{
				{
					Ipv4Address: to.StringPtr(mypodip),
				},
			},
		},
	}

	rrs, err := vdnsClient.CreateOrUpdate(context.Background(),
		azureresourcegroup,
		config.Kubernetes.Domainzone, config.Kubernetes.Domainprefix, dns.A, rs, "", "")

	if err != nil {
		return err
	}
	rrsmeta := *rrs.RecordSetProperties.Metadata
	for _, v := range *rrs.RecordSetProperties.ARecords {
		log.Info("Actual IP Address: " + *v.Ipv4Address + " Changed at: " + *rrsmeta["lastchangehuman"])
	}

	return nil
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

	f := flgs{}

	f.c = flag.String("c", "/config/bosnd.yml", "config file including path")
	f.b = flag.Bool("b", false, "Print the version and exit")

	flag.Parse()

	if *f.b == true {

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

	configfile = *f.c
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

	// create a cron job
	if config.Cron.Crontab != "" {
		c := cron.New()
		c.AddFunc(config.Cron.Crontab, func() {
			reloadprocess(config)
			log.Info("Sevice reloaded by cronjob")
		})
		c.Start()
		log.Info("Crontab controled reload started!")
	}

	for mainloop == true {

		// update Azure DNS with pod ip if enabled
		if config.Kubernetes.Updateazuredns == true {
			err := azuredns(config)
			if err != nil {
				log.Warn(aurora.Red(err))
				time.Sleep(time.Duration(config.Checkintervall) * time.Second)
				continue
			}
		}

		// reread config file
		ok := ReReadConfigfile(configfile, config)
		if !ok {
			log.Warn(aurora.Red("Error during config parsing, yet continuing!"))
			time.Sleep(time.Duration(config.Checkintervall) * time.Second)
			continue
		}

		log.Debug("Kubernetes namespace: " + config.Kubernetes.Namespace)
		if len(config.Swarm.Networks) == 0 && config.Kubernetes.Namespace == "" {
			log.Warn(aurora.Red("Swarm Network OR Kubernetes Namespace not configured"))
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

		if config.Kubernetes.Namespace != "" {
			// get the pods of the namespace ordered by label app=
			err := getkubernetespods(config)
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
