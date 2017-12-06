# Bosnd

Bosnd, the boatswain daemon. Dynamic, Docker Swarm based configuration file templating for all kinds of services (Apache httpd, Nginx, HAProxy, ...) made real. ```Bosnd``` means boatswain daemon. The word itself comes from ```bos'n``` which is a pronounciation used by people with sea legs.

## What it does

```Bosnd``` takes a configuration file as argument and based on that configuration file, it uses the given Docker Swarm connection to retrieve information from the Docker Swarm. Therefore the daemon connects to a Docker Swarm manager (leader or promoted node) via the Docker API. The needed information is collected from the ```docker network inspect -v <network>``` and ```docker service inspect <service>``` commands via API. After the information is retrieved, ```Bosnd``` processes the configured Golang templates and writes the resulting configuration files to the desired (configured) locations. Afterwards ```Bosnd``` will reload the controlled daemon, which is also configured in the ```bosnd.yml``` config file.

## Features

- Mutliple configuration templates
- Usable with any daemon binary
- Automatically reload the daemon on every change (if supported by the daemon) 
- Reconfigurable during runtime
- Recognises new Docker stack services which are appearing on the Docker network on the fly
- Add new Docker networks to the Bosnd on the fly
- Optional switchable PDNS support for publishing the linked ip-address on the fly
- Optional switchable Prometheus metrics
- Optional switchable debug mode including Golang pprof interface


## Why don't use xyz...

```Bosnd``` is not meant to replace an already existing software. If you are familiar with Traefik for example, please use it. We have created ```Bosnd``` because we were in the need to configure software in the Docker environment in a dynamic way. For example, Traefik is already able to read the running services out of the Docker Swarm events. But for us, thats not all we need and also thats not enough. There are a lot of other services which are not Docker Swarm aware. Imagine the following situation: You have a myriad of Apache Tomcat application servers as backends and they are working perfectly in combination with Apache httpd as a load balancer because the Apache httpd has a very pretty module called ```mod_jk```. Now, if you want to modernize this applications with the Docker Swarm environment, you will face some problems. First, you won't drop Apache httpd, because it is the best for the job. Therefore you need a flexible way to change the configuration of the httpd on the fly and reload it. Also, you won't like to map the Docker socket into the loadbalancer container to read the Docker events because that puts the system on a risk. If something terrible happens, someone can have access to the full Docker Swarm API. Bad. It's better to communicate with the swarm via the external API, the API can be protected by RBAC mechanisms like [Golang casbin](https://github.com/casbin/casbin-authz-plugin). And, last but not least, you may have more than one service like the Apache httpd which you would like to empower with dynamic template based configuration.

## And what about confd?

As we wrote ```Bosnd``` we soon recognized, that we are writing something like [confd](https://github.com/kelseyhightower/confd). Thats true. But with a different approach. confd will propagate the affected daemon with the new configuration and then reload/restart it. ```Bosnd``` in the opposite will be the number one process inside the container. It is responsible for the invoked daemon. If ```Bosnd``` dies, the container dies and it will be restarted (if configured in the compose-swarm.yml). Yes, there is always a discussion if there should be only one process inside a container but this is the decision we made (more than one process) for us because we need a general purpose tool. Next, instead of enforcing the possible combination of labels (like Traefik does;no critics here), we decided to let you label the things you like it because we can use the full capability of the Golang Template language. If you need the port, add a label and then use it in the template.

## But it is not real time...

... you are not listening on the Docker events!!! ;-)

Please define real time. Real time in computing is, when you can guarantee a system response within a specific time frame. By default, ```Bosnd``` will pull the actual state of the Docker network every 30 seconds (that's configurable). This is real time.

## Examples

You can find various run examples in the ```examples``` (like our first one: [apache-httpd](https://github.com/n0r1sk/bosnd/tree/master/examples/apache-httpd)) folder in the source code directory. You will have to run a command like ```./Bosnd -c <myBosndconfigfile>```. The ```Bosnd``` example configuration files are located within the example directories, including a Dockerfile and so on. Most of the examples will include a ```demo.sh``` file, which will help you to get used to the idea of the ```Bosnd```. The example directories will include a ```README.md``` too, read it! You can find a lot of information there.

## Docker certificates
For now the Docker CA certificate, the client certificate & client key files need to be exactly named as ```ca.pem```, ```client.pem``` & ```key.pem``` in the Docker certificate folder. Only this folder is configurable via the ```bosnd.yml```.

For example:
```
swarm:
  certificate: /docker/certs
```
In the next release ([see this issue](https://github.com/n0r1sk/bosnd/issues/1)) you will be able to define those three files in the ```bosnd.yml``` separately.

## Template files

The template files are working with the [Golang template language](https://golang.org/pkg/text/template/).

## Prometheus metrics

To enable the Prometheus metrics, use the following ```bosnd.yml``` configuration block. Replace the values enclosed by ```<>``` with your needs.

```
prometheus:
  start: true
  listenport: <yourlistenport>
```

Afterwards you can retreive the Prometheus metrics under ```http://<containerip>:<configuredport>/metrics```. The url heighly depends on your Docker stack configuration. If you expose the Prometheus port, the url will be different. If you are deploying a Prometheus consuming container with your stack you have to use the correct url accordingly.

The Prometheus implementation will provide all Golang metrics out of the box. In addtion we created an ```counter``` item, which provides the information about how often a new configuration was written (a configuration change with a reload happens).

```
# HELP bosnd_service_configuration_reloads The count of the configuration reloads since start.
# TYPE bosnd_service_configuration_reloads counter
bosnd_service_configuration_reloads 0
```

## PDNS

The PDNS implementation can be used to create an DNS A-record based on the configured parameters inside the ```Bosnd```. This enables you to move the ```Bosnd``` driven service from one ip-address to another. It is not fully perfect at the moment but usable. Replace the values enclosed by ```<>``` with your needs. Here is the ```bosnd.yml``` configuration block.

```
pdns:
  apiurl: https://<yourpdnserver>/api/v1/servers/localhost/zones
  apikey: <yourapikey>
  ipaddress: <ipaddressofbosndcontainer>
  domainprefix: <www>
  domainzone: <yourdomain.com>
```

## Control

The control interface can be used to force a service reload from the outside via http. This is useful, if you have services which are rely on SSL certificates which are renewed by a LetsEncrypt container. If this container renews the certificates you might want to reload the service which is controlled by the ```Bosnd``` too.
```
control:
  port: 3333
  key: mykey
```

If you enable the control block, you can use a browser (or curl) afterwards to trigger the reload: 
```
$ curl http://127.0.0.1:3333/reload/mykey
$ Reloaded!
```
