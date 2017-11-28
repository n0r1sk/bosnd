# bosnd

bosnd, the boatswain daemon. Dynamic, Docker Swarm based configuration file templating for all kind of services (Apache httpd, Nginx, HAProxy, ...) made real. ```bosnd``` means boatswain daemon. The word itself cames from ```bos'n``` which is a pronounciation used by people with sea legs.

## What it does

```bosnd``` takes a configuration file as argument and based on the configuration file, it uses the given Docker Swarm connection to retrieve information from the Docker Swarm. Therefore the daemon connects to the Docker Swarm manger (leader or promoted node) Docker API. The information needed is collected from the ```docker network inspect -v <network>``` command. After the information is retrieved, ```bosnd``` processes the configured Golang templates and write the resulting configuration files to the disired location. Afterwards ```bosnd``` will reload the controlled daemon, which is also configured in the ```bosnd.yml``` config file.

## Why don't use xyz...

```bosnd``` is not meant to replace an already existing software. If you are familar with Traefik, please use it. We have created ```bosnd``` because we were in the need to configure software in the Docker environment in a dynamic way. For example, Traefik is already able to read the running services out of the Docker Swarm events. But for us, thats not all we need and also thats not enough. Imagine the following situation: You have a myriad of Apache Tomcat application servers as backends and they are working perfect in combination with Apache httpd as loadbalancer because the Apache httpd has a very pretty module called ```mod_jk```. Now, if you want to modernize this applications with the Docker Swarm environment, you will face some problems. First, you wont drop Apache httpd away, because it does to job best. Therefore you need a flexible way to change the configuration of the httpd on the fly and reload it. Also, you won't like to map the Docker socket in the loadbalancer to read the Docker events. That puts the system on risk. If something terrible happens, someone can have access to the full Docker Swarm API. Bad. It's better to commuicate with the swarm via the external API, the API can be protected by RBAC mechanism like [Golang casbin](https://github.com/casbin/casbin-authz-plugin). An last but not least, you may have more than one service like the Apache httpd which you would like to empower with dynamic template based configuration.

## And what about confd?

As we wrote ```bosnd``` we soon recognized, that we are writing something like [confd](https://github.com/kelseyhightower/confd). Thats true. But with a different approach. confd will propagate the affected daemon with the new configuration and the reload/restart it. ```bosnd``` in the opposite will be the process number one inside the container, the process with the number 1. He is the responsible for the invoked daemon. If ```bosnd``` dies, the container dies and it will be restarted. Yes, there is a lot of discussion about one ore more processes inside a container but this is the desicion we made for us because we need a general purpose tool. Next, instead of forcing the labels you can use like Traefik does (no critic here), we decide to let you label things you like it because, instead of Traefik we can use the full capability of the Golang Template language. 

## But it is not real time...

... you are not listening on the Docker events!!!

Please correctly define real time! Real time in computing is, when you can gurantee a system respone whitin a specific time frame. By default, ```bosnd``` will pull the acutal state of the Docker network every 30 seconds. This is real time.

## Examples

You can find various run examples in the ```examples``` folder in the source code directory. But you will ever have to run a command like ```./bosnd -c <mybosndconfigfile>```. The ```bosnd``` example configuration files are located within the example directories. Most of the examples will include a ```demo.sh``` file, which will help you to get used to the idea of the ```bosnd```. The example directories will include a ```README.md``` too, read it! You can find a lot of information there.

## Template files
The template files are working with the [Golang template language](https://golang.org/pkg/text/template/).