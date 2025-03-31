# SPL Token Transfer

Basic example.

- No Durable Nonces, transactions must be broadcast less than 60s after being created
- Private key is stored locally - path is hardcoded as `signerKeyPath`
- Program ID is hardcoded as `programIDBase58` set to the token ID that you are interactiong with
