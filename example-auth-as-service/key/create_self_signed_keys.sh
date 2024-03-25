# reference : https://linuxize.com/post/creating-a-self-signed-ssl-certificate/
openssl req -newkey rsa:4096 \
            -x509 \
            -sha256 \
            -days 3650 \
            -nodes \
            -out rest-app.crt \
            -keyout rest-app.key \
            -subj "/C=US/ST=State/L=City/O=Organization/OU=Organization Unit/CN=www.rest-app.com"