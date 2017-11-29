# bosnd

bosnd, the boatswain daemon. Dynamic, Docker Swarm based configuration file templating for all kinds of services (Apache httpd, Nginx, HAProxy, ...) made real. ```bosnd``` means boatswain daemon. The word itself comes from ```bos'n``` which is a pronounciation used by people with sea legs.

## What it does

```bosnd``` takes a configuration file as argument and based on that configuration file, it uses the given Docker Swarm connection to retrieve information from the Docker Swarm. Therefore the daemon connects to a Docker Swarm manager (leader or promoted node) via the Docker API. The needed information is collected from the ```docker network inspect -v <network>``` and ```docker service inspect <service>``` commands via API. After the information is retrieved, ```bosnd``` processes the configured Golang templates and writes the resulting configuration files to the desired (configured) locations. Afterwards ```bosnd``` will reload the controlled daemon, which is also configured in the ```bosnd.yml``` config file.

## Why don't use xyz...

```bosnd``` is not meant to replace an already existing software. If you are familiar with Traefik for example, please use it. We have created ```bosnd``` because we were in the need to configure software in the Docker environment in a dynamic way. For example, Traefik is already able to read the running services out of the Docker Swarm events. But for us, thats not all we need and also thats not enough. There are a lot of other services which are not Docker Swarm aware. Imagine the following situation: You have a myriad of Apache Tomcat application servers as backends and they are working perfectly in combination with Apache httpd as a load balancer because the Apache httpd has a very pretty module called ```mod_jk```. Now, if you want to modernize this applications with the Docker Swarm environment, you will face some problems. First, you won't drop Apache httpd, because it is the best for the job. Therefore you need a flexible way to change the configuration of the httpd on the fly and reload it. Also, you won't like to map the Docker socket into the loadbalancer container to read the Docker events because that puts the system on a risk. If something terrible happens, someone can have access to the full Docker Swarm API. Bad. It's better to communicate with the swarm via the external API, the API can be protected by RBAC mechanisms like [Golang casbin](https://github.com/casbin/casbin-authz-plugin). And, last but not least, you may have more than one service like the Apache httpd which you would like to empower with dynamic template based configuration.

## And what about confd?

As we wrote ```bosnd``` we soon recognized, that we are writing something like [confd](https://github.com/kelseyhightower/confd). Thats true. But with a different approach. confd will propagate the affected daemon with the new configuration and then reload/restart it. ```bosnd``` in the opposite will be the number one process inside the container. It is responsible for the invoked daemon. If ```bosnd``` dies, the container dies and it will be restarted (if configured in the compose-swarm.yml). Yes, there is always a discussion if there should be only one process inside a container but this is the decision we made (more than one process) for us because we need a general purpose tool. Next, instead of enforcing the possible combination of labels (like Traefik does;no critics here), we decided to let you label the things you like it because we can use the full capability of the Golang Template language. If you need the port, add a label and then use it in the template.

## But it is not real time...

... you are not listening on the Docker events!!! ;-)

Please define real time. Real time in computing is, when you can guarantee a system response within a specific time frame. By default, ```bosnd``` will pull the actual state of the Docker network every 30 seconds (that's configurable). This is real time.

## Examples

You can find various run examples in the ```examples``` folder in the source code directory. You will have to run a command like ```./bosnd -c <mybosndconfigfile>```. The ```bosnd``` example configuration files are located within the example directories, including a Dockerfile and so on. Most of the examples will include a ```demo.sh``` file, which will help you to get used to the idea of the ```bosnd```. The example directories will include a ```README.md``` too, read it! You can find a lot of information there.

## Template files
The template files are working with the [Golang template language](https://golang.org/pkg/text/template/).
