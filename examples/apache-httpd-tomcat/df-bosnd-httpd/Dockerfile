FROM httpd:2.4.29

LABEL maintainer="mario.kleinsasser@gmail.com"
LABEL maintainer="rausch.bernhard@gmail.com"

# Install mod_jk distribution module
RUN apt-get update && apt-get -y install libapache2-mod-jk && cp /usr/lib/apache2/modules/mod_jk.so /usr/local/apache2/modules

RUN mkdir -p /bosnd/sites-templates && mkdir -p /bosnd/sites-enabled

# Enable Apache httpd ProxyPass reverse proxy
RUN sed -i '/^#.*mod_proxy.so/s/^#//' /usr/local/apache2/conf/httpd.conf && \
  sed -i '/^#.*mod_proxy_http.so/s/^#//' /usr/local/apache2/conf/httpd.conf && \
  sed -i '/^#.*mod_proxy_balancer.so/s/^#//' /usr/local/apache2/conf/httpd.conf && \
  sed -i '/^#.*mod_slotmem_shm.so/s/^#//' /usr/local/apache2/conf/httpd.conf && \
  sed -i '/^#.*mod_lbmethod_byrequests.so/s/^#//' /usr/local/apache2/conf/httpd.conf && \
  echo "Include /bosnd/sites-enabled/*.conf" >> /usr/local/apache2/conf/httpd.conf && \
  echo "LoadModule    jk_module  modules/mod_jk.so" >> /usr/local/apache2/conf/httpd.conf

# stdout for mod_jk.log
RUN ln -sf /proc/1/fd/1 /var/log/mod_jk.log

COPY bosnd /bosnd/bosnd
COPY bosnd.yml /bosnd/bosnd.yml
COPY site.template /bosnd/sites-templates
COPY workers.template /bosnd/sites-templates

CMD ["/bosnd/bosnd", "-c", "/bosnd/bosnd.yml"]