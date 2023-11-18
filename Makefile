HOST_NAME='/CN=127.0.0.1'

certs:
	openssl genrsa -out ./crt/server.key 4096
	openssl req -x509 -key ./crt/server.key -out ./crt/server.crt -days 365 -subj ${HOST_NAME}


