FROM httpd:2.4.29

LABEL maintainer="mario.kleinsasser@gmail.com"
LABEL maintainer="rausch.bernhard@gmail.com"

RUN mkdir -p /bosnd/sites-templates && mkdir -p /bosnd/sites-enabled
RUN sed -i '/^#.*mod_proxy.so/s/^#//' /usr/local/apache2/conf/httpd.conf && \
  sed -i '/^#.*mod_proxy_http.so/s/^#//' /usr/local/apache2/conf/httpd.conf && \
  sed -i '/^#.*mod_proxy_balancer.so/s/^#//' /usr/local/apache2/conf/httpd.conf && \
  sed -i '/^#.*mod_slotmem_shm.so/s/^#//' /usr/local/apache2/conf/httpd.conf && \
  sed -i '/^#.*mod_lbmethod_byrequests.so/s/^#//' /usr/local/apache2/conf/httpd.conf && \
  echo "Include /bosnd/sites-enabled" >> /usr/local/apache2/conf/httpd.conf

COPY bosnd /bosnd/bosnd
COPY certs /bosnd/certs
COPY bosnd.yml /bosnd/bosnd.yml
COPY site.template /bosnd/sites-templates

CMD ["/bosnd/bosnd", "-c", "/bosnd/bosnd.yml"]
