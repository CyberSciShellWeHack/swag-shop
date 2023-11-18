# Swag shop API

This service represents the backend API for a SWAG shop. The shop manages the sale of event swag.

## Functionality

There is an admin only API which can do the following:

- Create item (POST /items)
- Delete item (DELETE /items/{id})
- List orders (GET /orders)

The admin has a Elliptic Curve private key. This key will never change. 

To authenticate the admin must:

1. Get the current auth message (GET /auth)
2. Sign the SHA 256 hash of the current message using their private key (sent as a hex string)

The shop server will use the corresponding public key to verify that the signature is valid for the current auth message.

Without any auth you can do: 

- List items (GET /items)
- Describe item (GET /items/{id})
- Buy an item (POST /items/{id})

## Deployment

To run the service you can use:

`docker-compose up -d`
`docker-compose restart`
`docker-compose down`
`docker-compose ps`

Upon making changes to the service you will need to rebuild:

`docker-compose build`



