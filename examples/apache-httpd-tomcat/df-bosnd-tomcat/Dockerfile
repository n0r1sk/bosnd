FROM tomcat:8.5.24-jre8

LABEL maintainer="mario.kleinsasser@gmail.com"
LABEL maintainer="rausch.bernhard@gmail.com"

# Add JAVA_OPTS to catalina.sh to enable mod_jk sticky workers
RUN sed -i '60i JAVA_OPTS="-DjvmRoute=$HOSTNAME"' /usr/local/tomcat/bin/catalina.sh


# Copy demo app to webapps folder
COPY demo.war /usr/local/tomcat/webapps/demo.war