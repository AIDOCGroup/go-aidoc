

package main

import (
	"testing"
	"os"
	"gopkg.in/urfave/cli.v1"
	"github.com/aidoc/go-aidoc/lib/logger"
	"path/filepath"
	"github.com/aidoc/go-aidoc/lib/i18"
)

func TestRun(t *testing.T)  {
	//app.RunAndExitOnError();
	args :=os.Args[:];
	logger.Info( i18.I18_print.Sprintf("%v", args ))

	os_args :=args[0:1]

	//os_args = append(os_args , "--rinkeby")
	os_args = append(
				append(os_args , "--datadir"),
				filepath.Dir("./") + "/data/store/" )

	os_args =	append(os_args , "-networkid"  , "1")

	os_args =	append(os_args , "--rpccorsdomain" , "*" )
	os_args = 	append(os_args , "--rpcapi" , "net,aidoc,web3,personal" )
	os_args = 	append(os_args , "--rpc")
	os_args = 	append(os_args , "--fast")
	os_args = 	append(os_args , "--port", "30603")
	os_args = 	append(os_args , "--rpcport", "8645")
	os_args = 	append(os_args , "--ws")
	os_args = 	append(os_args , "--wsport" , "8646")
	os_args = append( os_args , "console")
	//os_args = append(os_args , "--cache=1024")
	if err := app.Run(os_args); err != nil {
		logger.Crit( err.Error())
	}
	os.Exit(0)

}

func TestInitGenesis(t *testing.T)  {
	//app.RunAndExitOnError();
	args :=os.Args[:];
	logger.Info( i18.I18_print.Sprintf("%v", args ))

	os_args :=args[0:1]

	//os_args = append(os_args , "--rinkeby")
	os_args = append( os_args , "init")
	os_args = append( os_args , "/Users/long/go-path/src/github.com/aidoc/go-aidoc/cmd/gad/testdata/genesis.json")
	os_args = append(
		append(os_args , "--datadir"),
		"/Users/long/data/aidoc")


	if err := app.Run(os_args); err != nil {
		logger.Crit( err.Error())
	}
	os.Exit(0)

}


func TestCli(t *testing.T){
	app := cli.NewApp()
	app.Action = func(c *cli.Context) error {
		i18.I18_print.Println("BOOM!")
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		logger.Crit(err.Error())
	}

}