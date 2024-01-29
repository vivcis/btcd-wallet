package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"os"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
)

func createP2PKHWallet() (*btcutil.AddressPubKeyHash, *btcec.PrivateKey, error) {
	keyPair, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		return nil, nil, err
	}

	// Derive the P2PKH address from the public key
	pkHash := btcutil.Hash160(keyPair.PubKey().SerializeCompressed())
	address, err := btcutil.NewAddressPubKeyHash(pkHash, &chaincfg.TestNet3Params)
	if err != nil {
		return nil, nil, err
	}

	// Create a wallet struct
	wallet := struct {
		Address    string `json:"address"`
		PrivateKey string `json:"private_key"`
	}{
		Address:    address.EncodeAddress(),
		PrivateKey: hex.EncodeToString(keyPair.Serialize()),
	}

	// Convert wallet struct to JSON
	walletJSON, err := json.MarshalIndent(wallet, "", "    ")
	if err != nil {
		return nil, nil, err
	}

	// Write JSON to wallet.json file
	err = ioutil.WriteFile("wallet.json", walletJSON, 0644)
	if err != nil {
		return nil, nil, err
	}

	return address, keyPair, nil
}

func sendBitcoin(fromAddress *btcutil.AddressPubKeyHash, privKey *btcec.PrivateKey, toAddress btcutil.Address, amount btcutil.Amount) error {
	client, err := rpcclient.New(&rpcclient.ConnConfig{
		HTTPPostMode: true,
		DisableTLS:   true,
		Host:         os.Getenv("RPC_HOST"),
		User:         os.Getenv("RPC_USER"),
		Pass:         os.Getenv("RPC_PASSWORD"),
	}, nil)
	if err != nil {
		return err
	}
	defer client.Shutdown()

	// List unspent outputs (UTXOs) for the fromAddress
	unspentOutputs, err := client.ListUnspentMinMaxAddresses(1, 9999999, []btcutil.Address{fromAddress})
	if err != nil {
		return err
	}

	if len(unspentOutputs) == 0 {
		return fmt.Errorf("no unspent outputs found for the given address")
	}

	// Create a transaction
	tx := wire.NewMsgTx(wire.TxVersion)

	// Convert utxo.TxID (string) to *chainhash.Hash
	txID, err := chainhash.NewHashFromStr(unspentOutputs[0].TxID)
	if err != nil {
		return err
	}

	// Create a transaction input (UTXO)
	prevOut := wire.NewOutPoint(txID, unspentOutputs[0].Vout)
	txIn := wire.NewTxIn(prevOut, nil, nil)
	tx.AddTxIn(txIn)

	// Add the transaction output
	pkScript, err := txscript.PayToAddrScript(toAddress)
	if err != nil {
		return err
	}

	txOut := wire.NewTxOut(int64(amount), pkScript)
	tx.AddTxOut(txOut)

	// Sign the transaction
	sigScript, err := txscript.SignatureScript(tx, 0, []byte(unspentOutputs[0].ScriptPubKey), txscript.SigHashAll, privKey, true)
	if err != nil {
		return err
	}

	txIn.SignatureScript = sigScript

	// Send the transaction
	txHash, err := client.SendRawTransaction(tx, false)
	if err != nil {
		return err
	}

	fmt.Printf("Transaction sent successfully! Transaction Hash: %s\n", txHash.String())
	return nil
}

func getBalance(address *btcutil.AddressPubKeyHash) (btcutil.Amount, error) {
	client, err := rpcclient.New(&rpcclient.ConnConfig{
		HTTPPostMode: true,
		DisableTLS:   true,
		Host:         os.Getenv("RPC_HOST"),
		User:         os.Getenv("RPC_USER"),
		Pass:         os.Getenv("RPC_PASSWORD"),
	}, nil)
	if err != nil {
		return 0, err
	}
	defer client.Shutdown()

	// List unspent outputs (UTXOs) for the address
	unspentOutputs, err := client.ListUnspentMinMaxAddresses(1, 9999999, []btcutil.Address{address})
	if err != nil {
		return 0, err
	}

	// Calculate the total balance from the list of unspent outputs
	var totalBalance btcutil.Amount
	for _, output := range unspentOutputs {
		// Convert to satoshis
		totalBalance += btcutil.Amount(output.Amount * 1e8)
	}

	return totalBalance, nil
}

func main() {
	//read env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// Create a wallet
	fromAddress, privKey, err := createP2PKHWallet()
	if err != nil {
		log.Fatal(err)
	}

	// Display wallet information
	fmt.Printf("| Public Address | %s |\n", fromAddress.EncodeAddress())
	fmt.Printf("| Private Key | %s |\n", hex.EncodeToString(privKey.Serialize()))

	// Send Bitcoin to another address
	toAddress, err := btcutil.DecodeAddress("mn96nX5NkZfrMmCV7TWQiNKfhgLM6VYQyY", &chaincfg.TestNet3Params)
	if err != nil {
		log.Fatal(err)
	}

	err = sendBitcoin(fromAddress, privKey, toAddress, btcutil.Amount(1000))
	if err != nil {
		log.Fatal(err)
	}

	// Check the wallet's balance
	balance, err := getBalance(fromAddress)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Wallet Balance: %s BTC\n", balance.String())

}
