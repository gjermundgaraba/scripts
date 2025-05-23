const { ethers } = require('ethers');
const fs = require('fs');

// Generate a random mnemonic (BIP39)
const mnemonic = ethers.Wallet.createRandom().mnemonic.phrase;

// Derive wallet from mnemonic
const wallet = ethers.Wallet.fromPhrase(mnemonic);

const fileName = 'out/mnemonic_gen.txt';
const content = `Mnemonic: ${mnemonic}\nPrivate Key: ${wallet.privateKey}\nAddress: ${wallet.address}\n`;
fs.writeFileSync(fileName, content)

console.log('Mnemonic and wallet information saved to', fileName);

