package main

//
//
//import (
//	"fmt"
//	"log"
//	"os"
//	"os/signal"
//	"strings"
//	"syscall"
//
//	"github.com/dimaskiddo/codebase-go-rest/pkg/cache"
//	"github.com/dimaskiddo/codebase-go-rest/pkg/db"
//	"github.com/dimaskiddo/codebase-go-rest/pkg/router"
//	"github.com/dimaskiddo/codebase-go-rest/pkg/server"
//
//	"github.com/dimaskiddo/codebase-go-rest/internal"
//)
//
//// Server Variable
//var svr *server.Server
//
//// Init Function
//func init() {
//	// Set Go Log Flags
//	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
//
//	// Load Routes
//	internal.LoadRoutes()
//
//	// Initialize Server
//	svr = server.NewServer(router.Router)
//}
//
//// Main Function
//func main() {
//	// Starting Server
//	svr.Start()
//}
