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
	"encoding/json"
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
	"strings"
	"syscall"
	"text/template"
	"time"

	"golang.org/x/net/context"

	// golang profiling
	_ "net/http/pprof"

	// colorize output
	"github.com/logrusorgru/aurora"

	// cron library
	"github.com/robfig/cron"

	// prometheus interface
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	// kubernetes client
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// etcd client
	ec "github.com/coreos/etcd/client"

	// logging
	log "github.com/sirupsen/logrus"

	// commandline command and flag parsing
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var mainloop bool
var ctrlcmd *exec.Cmd
var configfile string

// Global ETCD client
var etcdclient ec.KeysAPI

// Global Kubernetes client
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

var versionTemplate = `The Kube-Bosnd - made by https://www.n0r1sk.com
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

func getkubernetesclient(config *Config) bool {

	log.Debug("Testing Kubeclient")
	ok := false

	for ok == false {
		var err error

		// use the current context in kubeconfig
		kconfig, err := clientcmd.BuildConfigFromFlags("", config.Kubernetes.Kubeconfig)
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

	if config.Kubernetes.Kubeconfig == "" {
		return errors.New("Kuberntes client config not set in config")
	}

	ok := getkubernetesclient(config)
	if ok != true {
		return errors.New("Failed to create Kubernetes Client")
	}

	var pods *v1.PodList
	var err error
	if config.Kubernetes.Labelselector != "" {
		pods, err = kubeclient.CoreV1().Pods(config.Kubernetes.Namespace).List(metav1.ListOptions{LabelSelector: config.Kubernetes.Labelselector})
	} else {
		pods, err = kubeclient.CoreV1().Pods(config.Kubernetes.Namespace).List(metav1.ListOptions{})
	}

	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

	var services map[string][]Pod
	// init map
	services = make(map[string][]Pod)

	for _, p := range pods.Items {

		if p.Labels["kbds"] == "" {
			continue
		}

		// create pod
		var po Pod
		po.Labels = p.Labels
		po.Hostname = p.Name
		po.Address = p.Status.PodIP

		// add to pod to app
		services[p.Labels["kbds"]] = append(services[p.Labels["kbds"]], po)

		log.Debug(po.Hostname + " " + po.Address)
	}

	for _, pods := range services {
		sort.Slice(pods, func(i, j int) bool {
			return pods[i].Hostname < pods[j].Hostname
		})
	}

	config.Kubernetes.Services = &services
	log.Debug(config.Kubernetes.Services)

	return nil
}

func corednsset(config *Config) error {
	// check if the coredns path is set
	if config.Coredns.Path == "" {
		return errors.New("coredns path in config file not set")
	}

	// check if the Arecord DNS name is set
	if config.Coredns.Arecord == "" {
		return errors.New("coredns A-record in config file not set")
	}

	// preapare domainname for etcd
	sdomain := strings.Split(config.Coredns.Arecord, ".")
	var dnsrecord string
	dnsrecord = dnsrecord + config.Coredns.Path

	for i := len(sdomain) - 1; i >= 0; i-- {
		dnsrecord = dnsrecord + "/" + sdomain[i]
	}

	// prepare pod ip of loadbalancer
	host := os.Getenv("KUBE_BOSND_POD_IP")
	if host == "" {
		return errors.New("environment variable KUBE_BOSND_POD_IP not set")
	}

	// prepare ttl of record
	ttl := config.Coredns.TTL
	if ttl == 0 {
		ttl = 60
	}

	// prepare record
	var r Record
	r.Host = host
	r.TTL = ttl
	log.Debug(r)

	b, err := json.Marshal(r)
	if err != nil {
		log.Debug("error during json marshal of record")
		return err
	}

	value := string(b[:])
	log.Debug(value)

	// set "/foo" key with "bar" value
	resp, err := etcdclient.Set(context.Background(), dnsrecord, value, nil)
	if err != nil {
		log.Debug("error during etcd set of record")
		return err
	}

	// print common key info
	log.Infof("Set is done. Metadata is %q", resp)

	// get "/foo" key's value
	resp, err = etcdclient.Get(context.Background(), dnsrecord, nil)
	if err != nil {
		return err
	}

	// print common key info
	log.Infof("Get is done. Metadata is %q", resp)

	// print value
	log.Infof("%q key has %q value", resp.Node.Key, resp.Node.Value)

	return nil
}

func prepareetcdclient(config *Config) error {

	endpoints := config.Coredns.Etcd
	clientcert := os.Getenv("COREDNS_ETCD_CERT")
	clientcertkey := os.Getenv("COREDNS_ETCD_CERTKEY")
	ca := os.Getenv("COREDNS_ETCD_CA")

	if clientcert == "" {
		return errors.New("environment variable COREDNS_ETCD_CERT not set")
	}
	if clientcertkey == "" {
		return errors.New("environment variable COREDNS_ETCD_CERTKEY not set")
	}

	if ca == "" {
		return errors.New("environment variable COREDNS_ETCD_CA not set")
	}

	// Load client cert
	cert, err := tls.LoadX509KeyPair(clientcert, clientcertkey)
	if err != nil {
		return err
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile(ca)
	if err != nil {
		return err
	}

	// Refresh the CA pool
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}

	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
		TLSClientConfig:    tlsConfig,
	}

	cfg := ec.Config{
		Endpoints: endpoints,
		Transport: tr,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}
	c, err := ec.New(cfg)
	if err != nil {
		return err
	}
	etcdclient = ec.NewKeysAPI(c)

	return nil

}

// VersionCommand Context for "ls" command
type VersionCommand struct {
}

// RunCommand runs the bosnd
type RunCommand struct {
	Config string
}

func (l *RunCommand) run(c *kingpin.ParseContext) error {
	if l.Config == "" {
		configfile = "/config/bosnd.yml"
	} else {
		configfile = l.Config
	}
	return nil
}

func (l *VersionCommand) run(c *kingpin.ParseContext) error {
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
	return nil
}

func configureVersionCommand(app *kingpin.Application) {
	c := &VersionCommand{}
	app.Command("version", "Show version.").Action(c.run)
}

func configureRunCommand(app *kingpin.Application) {
	c := &RunCommand{}
	run := app.Command("run", "Run the kube-bosnd").Action(c.run)
	run.Flag("config", "Specify the config file, default /config/bosnd.yml").Short('c').StringVar(&c.Config)
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

	// command line parsing
	app := kingpin.New("bosnd", "The boatswain daemon")
	configureVersionCommand(app)
	configureRunCommand(app)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	// log the configfile path
	log.Info(configfile)

	// parse the config file
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

	// log the configfile content
	log.Debug("Configfile Keys and Values: ", config)

	// check if Prometheus is enabled
	if config.Prometheus.Port != "" {
		go prom(config)
	}

	// only take the control interface into accout if it is configured
	if config.Control.Key != "" {
		go api(config)
	}

	// check if CoreDNS is enabled, prepare the ETCD client and set the DNS
	if len(config.Coredns.Etcd) != 0 {
		log.Info(aurora.Green("CoreDNS enabled"))
		err := prepareetcdclient(config)
		if err != nil {
			log.Debug("error during creation of ETCD client")
			log.Fatal(aurora.Red(err))
		}
		err = corednsset(config)
		if err != nil {
			log.Debug("error during COREDNS set")
			log.Fatal(aurora.Red(err))
		}
	}

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

		log.Debug("Kubernetes namespace: " + config.Kubernetes.Namespace)
		if config.Kubernetes.Namespace == "" {
			log.Warn(aurora.Red("Kubernetes Namespace not configured"))
			time.Sleep(time.Duration(config.Checkintervall) * time.Second)
			continue
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
