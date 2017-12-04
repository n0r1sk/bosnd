# Demo

**Conclusion: This is for demo purpose only! Do not use it this way in production!**

# PWD Play with docker

[![Try in PWD](https://cdn.rawgit.com/play-with-docker/stacks/cff22438/assets/images/button.png)](http://play-with-docker.com/?stack=https://raw.githubusercontent.com/n0r1sk/bosnd/master/examples/apache-httpd-tomcat/docker-stack-pwd.yml)

After the stack has been started, click on the **80** visible in the PWD UI. This will open a new browser tab. There you will see the **It work's** text from the Apache httpd. This is perfect, as it shows that the Apache httpd has started. Now append the following to the url in the browser window: ```/demo/versiontest.jsp```, for example ```.../demo/versiontest.jsp``` This will open the demo webapplication from the Apache Tomcat which we have included. It will show your session id and a session access count. If you open up a second, **independed** alternative browser and paste the url from the first one, you will get a new session there. Now refresh the page multiple times. You will notice, that you stay in the same session. Voila!

The Docker stack will start the ```Bosnd``` prepared Apache httpd and two additional Apache Tomcat workers. It will configure itself flawlessly! Scale the ```docker service scale pwd_tomcat=4``` and see the magic! Add ```/jkmanager``` to the url in the browser to access the jkstatus page of the ```mod_jk``` module, for example ```direct.labs.play-with-docker.com/jkmanager```. After a few seconds, the additional worker will come up there! (see screenshot two)

## Screenshots

![screenshot](https://raw.githubusercontent.com/n0r1sk/bosnd/master/examples/apache-httpd-tomcat/screenshot.jpg)

![screenshot](https://raw.githubusercontent.com/n0r1sk/bosnd/master/examples/apache-httpd-tomcat/screenshot2.jpg)

# The anatomy

Here you can find a brief description and explanation what the components are doing and why it works how it works. First we will give you some tips and tricks and additional insight, about why this works. The demo.sh allows you to build all you need for this demo locally too.

## Tips and tricks

We will start of by the ```docker-stack-pwd.yml```. This is, where all of the things start. This section will cover the ```Play with Docker``` version of the demo, but it will be the same for the local demo version.

### docker-stack-pwd.yml

Here is one very important line hidden: ```hostname: "{{.Service.Name}}-{{.Task.Slot}}"```. This line located in the ```tomcat``` service will ensure, that your Tomcat service containers will get dynamic but predictable hostnames. You will need this, as this value is used by the ```jvmRoute=$HOSTNAME``` parameter in the Tomcat ```catalina.sh``` script. This value must be reflected in the Apache httpd mod_jk workers configuration, and it must be exactly the same!

Be aware, that if you start a stack on PWD via a PWD-Link (like on this page), the Docker stack will be automatically named ```pwd_...```.

### df-bosnd-tomcat/Dockerfile

We are using the official Apache Tomcat image for this demo. Now, to set the ```jvmRoute=``` for the Apache httpd mod_jk loadbalancing in the Apache Tomcat image, we run the following command during the Docker build of the Docker image:

```RUN sed -i '60i JAVA_OPTS="-DjvmRoute=$HOSTNAME"' /usr/local/tomcat/bin/catalina.sh``` 

This will ensure that the ```JAVA_OPTS=``` parameter is correctly enabled in the ```catalina.sh``` file and uses the value of ```$HOSTNAME```. ```$HOSTNAME``` is set to a dynamic but predictable hostname ensured by the ```docker-stack-pwd.yml```. See the earlier section for details.

### df-hosnd-httpd/Dockerfile

Now as we have finished the Apache Tomcat Dockerfile, we need an appropriate Apache httpd with mod_jk enabled. Therefore the Dockerfile will include the ``` apt-get``` run to include the mod_jk module. As the offical Apache httpd images is build from source, we have to copy the ```mod_jk.so``` file to the correct location. **That is not the best way to achieve this!**, that is only for this demo! Next, we have to enable the needed Apache httpd modules in the httpd config file, which is done by multiple ```sed``` statements. Afterwards, we link the stdout ```/proc/1/fd/1``` to ```/var/log/mod_jk.log``` and use this configuration in the site.template and workers.template.

### df-hosnd-httpd/bosnd.yml

There is one new parameter there : ```usesocket: true```. This enforces the ```Bosnd``` to use the mounted Docker socket.

## demo.sh (documentation has to be changed from here for this example...)

With the demo.sh script, you are able to run an Apache httpd as cookie based reverse proxy for two Apache Tomcat backends, including sticky session load balancing!

This is the starting point of the demo. I (Mario) hate such scripts, because they are hiding the details from the user but in some cases it is necessary to provide a script, to get users onboard which are not familar with the in deep details. We will try to explain everything, because we want that you are able to follow the idea and to be able to build a service on your own. Let's start.

Here is the first line of the demo script. Check if we have Docker swarm mode.
```
if docker node ls > /dev/null 2>&1; then
  echo "Swarm mode enabled"
else
  echo "Swarm mode not enabled. Exiting!"; exit
fi
```

If you forget to place the client certificate for the Docker Swarm interface into the ```certs``` folder, the script will terminate. We decided to use the client certificate solution to secure the Docker API interface, as it is well documented. The ```Bosnd``` is only working if you are running the Docker API interface TLS secured. No compromise here. If you don't like it, feel free to fork the project and change the code for your needs.

```
if [ "$(ls -A certs)" ]; then
  echo "Certificate found."
else
  echo "No certficate in certs folder found? ca.pem? client.pem? key.pem? Exiting..."
  exit
fi
```

Next, we copy the ```Bosnd``` binary to the Docker build directory. This is, where the ```demo.sh``` is located, next to this ```README.md```

```
cp ../../bosnd .
```

And the next line will trigger the docker build. The resulting Docker image is tagged as ```bosnd-httpd:1.0``` because we must know the name later in the Docker Swarm file.

```
docker build . -t bosnd-httpd:1.0
```

After the build, we remove the ```Bosnd``` binary which we copied in the step before the build to not mess up our build folder.

```
rm bosnd
```

The next step is **important** and needs some kind of explanation. As you know, normally the Docker swarm will create a network automatically. This could be done in the Docker swarm compose file with the config parameter ```attachable: true``` in the network part. That is really OK, but there might be a pitfall. Imagine the following situation: You deploy a service, including the network as attachable. Now you join the ```bosnd``` controlled load balancer to the network to get your services routed. This may happen on a different Docker host, where you can have a lot of ip addresses. Due to maintenance reasons, you shutdown the load balancer and of course the associated Docker swarm service. The result is, that you cannot start the load balancer before the Docker stack because the network you would like to join has gone away. 
```
docker network create --attachable --driver=overlay bosnd_net
```

Next step, deploy the Docker stack.
```
docker stack deploy -c docker-stack.yml bosnd
```

Afterwards we wait for the user input.
```
sleep 2
echo -n "Docker stack started! Point your browser to http://127.0.0.1:3456! Press [ENTER] to stop the stack: "
read
```

And delete the Docker stack.
```
docker stack rm bosnd
```

## Dockerfile

The Dockerfile is pretty straight forward. There are some sed's in there to change the httpd.conf file to enable the modules needed for the ```ProxyPass``` functionality. Then the ```sites-template``` and the ```bosnd.yml``` configuration files are copied into the new image.

In the last step, the ```CMD``` will be changed to start the ```bosnd``` instead of the ```httpd```. Then ```httpd``` will later be configured, started and controlled through the ```Bosnd```.

## bosnd.yml

The ```bosnd.yml``` file is, where the magic happens. First, there is the ```debug``` switch which enables the debug mode. The ```cmd``` block is used to configure the binary which should be controlled by the ```Bosnd```. The ```start``` and ```reload``` parameters take string arrays as values. **Important:** You have to specify the ```processname``` which you can find in the ```proc``` filesystem. Golang has no build in feature to find the process. Just start your controlled binary, get the process number and look for the status information on it in the ```proc``` filesystem. Here is a small example how you can achieve this. The value of the ```Name:```.

```
# sleep 1000 &
[1] 7630
# cat /proc/7630/status | grep Name:
Name:   sleep
```

The ```templates``` block is used to specify one or more(!) template sources and destinations. A template is read from src, processed with the Docker swarm information and then written to the destination file. **Again, you can specify multiple files if needed**. For example, if you want to configure Apache httpd with modJK, you can add an additional template file for the workers.properties.

The ```swarm``` block will configure your access to the Docker Swarm API and what information you would like to retrieve. ```certificate``` is used as path to your Docker Swarm API client certificate. By specifying ```domainzone``` and ```domainprefix``` you can add your domain information to the configuration to access it later in the template.

```managerurl``` is the one of the important ones. Please specify a DNS name. Best case, specify a DNS name with multiple A-records for DNS round robin your managers! The ```Bosnd``` will try endlessly to get a connection to the ```managerurl```.

The second important one is ```networks```. Specify one ore more Docker network names, where your services are started and the ```Bosnd``` has access to. ```Bosnd``` will retrieve all information and later you will use this information in the template file.

## site.template

The site template uses the Golang template language. Please look into the offical documentation, how and why this works. One sentence: The Golang template language is extremely powerful but not that easy to understand. The map used for the data is documented in the ```types.go``` file in the source code. If you would like to see what you have, just copy the following into the template. It might not start your service, but in the destination file you can see the whole data structure!

```
{{.Services}}
```

## docker-swarm.yml

**Important:** Label your service(s) correctly to resolve them in the template!

## Manual run
```docker run -ti --rm -v /var/run/docker.sock:/var/run/docker.sock --network=bosnd_net -p 3456:80 bosnd-httpd:1.0 bash```
```docker service create --hostname "{{.Service.Name}}-{{.Task.Slot}}" --network bosnd_net --replicas 2 --label "context=/demo" --label "port=8009" bosnd-tomcat:1.0```
```http://localhost:3456/demo/versiontest.jsp```



And now, happy hacking.







