package common

import (
	"fmt"
	"os"
)

/*
	Variable to control output verbosity.
	Defaults to false
*/
var Verbose bool = false

/*
	Exits the application with the error code given after
	printing an error message string to the console.
*/
func ExitWithString(errorCode int, errorMessage string) {
	Print("[ERROR] %s\n", errorMessage)
	os.Exit(errorCode)
}

/*
	Exits the application with the error code given after
	printing an error message from an error object to console.
*/
func ExitWithError(errorCode int, err error) {
	Print("[ERROR] %s\n", err.Error())
	os.Exit(errorCode)
}

/*
	Exits the application with the error code given after
	printing a string and message from an error object to console.
*/
func ExitWithErrorAndString(errorCode int, err error, errorMessage string) {
	Print("[ERROR] %s\n", errorMessage)
	Print("[ERROR] %s\n", err.Error())
	os.Exit(errorCode)
}

/*
	Prints a message to the console, can take in arguments
	after the initial message so you can treat this as a Sprintf()
*/
func Print(message string, args ...interface{}) {
	fmt.Printf("[bugsnag-to-csv] "+message+"\r\n", args...)
}

/*
	Prints a message to the console, in the same way as Print()
	but ONLY if `Verbose` is set to true
*/
func PrintVerbose(message string, args ...interface{}) {
	if Verbose {
		fmt.Printf("[b2c----VERBOSE] "+message+"\r\n", args...)
	}
}

/*
	Prints a header to the console, including the application version
*/
func PrintHeader() {
	if Verbose {
		Print("##################################################")
		Print("#                                                #")
		Print("#              Bugsnag-to-CSV, v%s            #", PackageVersion)
		Print("#                Xander Jones, 2021              #")
		Print("# https://github.com/xander-jones/bugsnag-to-csv #")
		Print("#                                                #")
		Print("##################################################")
	}
}
