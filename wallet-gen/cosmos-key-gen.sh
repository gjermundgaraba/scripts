#!/bin/bash

# Directory to store output
OUTPUT_DIR="./out"
mkdir -p "$OUTPUT_DIR"

# Output file
OUTPUT_FILE="$OUTPUT_DIR/cosmos_wallets.toml"

# Initialize the output file
echo "# Cosmos Wallets" > "$OUTPUT_FILE"

for i in {13..100}
do
  key_name="a-wallet$i"

  # Generate the key
  gaiad keys add "$key_name" --keyring-backend test

  # Export the private key in unarmored hex format
  # --unsafe flag is required to export private keys
  priv_key_hex=$(echo "y" | gaiad keys export "$key_name" --unarmored-hex --unsafe --keyring-backend test)

  # Append to the output file in the requested format
  echo -e "\n[[wallets]]" >> "$OUTPUT_FILE"
  echo "wallet-id = \"cosmos-$i\"" >> "$OUTPUT_FILE"
  echo "private-key = \"$priv_key_hex\"" >> "$OUTPUT_FILE"

  # Output progress to the console
  echo "Added wallet cosmos-$i to $OUTPUT_FILE"
done

echo "All wallets have been generated and saved to $OUTPUT_FILE"
