const { ethers } = require('ethers');
const fs = require('fs');
const path = require('path');

// Create output directory if it doesn't exist
const outputDir = path.join(__dirname, 'out');
if (!fs.existsSync(outputDir)) {
  fs.mkdirSync(outputDir);
}

// Output file path
const outputFile = path.join(outputDir, 'eth_wallets.toml');

// Initialize the output file
fs.writeFileSync(outputFile, '# Ethereum Wallets\n');

// Generate wallets
for (let i = 13; i <= 100; i++) {
  const wallet = ethers.Wallet.createRandom();
  
  // Format the private key to remove the '0x' prefix if present
  const privateKey = wallet.privateKey.startsWith('0x') 
    ? wallet.privateKey.substring(2) 
    : wallet.privateKey;
  
  // Write wallet data to the file in the requested format
  const walletData = `
[[wallets]]
wallet-id = "eth-${i}"
private-key = "${privateKey}"
`;
  
  fs.appendFileSync(outputFile, walletData);
  
  // Log progress to console
  console.log(`Added wallet eth-${i} to ${outputFile}`);
}

console.log(`All wallets have been generated and saved to ${outputFile}`);
