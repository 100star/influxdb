package main

import (
	"flag"
	"log"

	"github.com/influxdb/influxdb/messaging"
)

// execJoinCluster runs the "join-cluster" command.
func execJoinCluster(args []string) {
	// Parse command flags.
	fs := flag.NewFlagSet("", flag.ExitOnError)
	var (
		configPath  = fs.String("config", "", "")
		role        = fs.String("role", "combined", "")
		seedServers = fs.String("seed-servers", "", "")
	)
	fs.Usage = printJoinClusterUsage
	fs.Parse(args)

	// Parse configuration.
	config, err := ParseConfigFile(*configPath)
	if err != nil {
		log.Fatalf("config: %s", err)
	}

	// Validate command line arguments.
	if *seedServers == "" {
		log.Fatal("at least one seed server must be supplied")
	} else if *role != "combined" && *role != "broker" && *role != "data" {
		log.Fatal("node must join as 'combined', 'broker', or 'data'")
	}

	// If joining as broker then create broker.
	if *role == "combined" || *role == "broker" {
		// Broker required -- but don't initialize it.
		// Joining a cluster will do that.
		b := messaging.NewBroker()
		if err := b.Open(config.Raft.Dir); err != nil {
			log.Fatalf("join: %s", err)
		}
	}

	// If joining as a data node then create a data directory.
	if *role == "combined" || *role == "data" {
		// TODO: do any required data-node stuff.
	}

	log.Printf("joined cluster at %s", *seedServers)
}

func printJoinClusterUsage() {
	log.Println(`usage: join-cluster [join-cluster and shared flags]

join-cluster creates a completely new node that will attempt to join an existing cluster.

        -role <role>
                        Set the role to be 'combined', 'broker' or 'data'. broker' means it will take
                        part in Raft Distributed Consensus. 'data' means it will store time-series data.
                        'combined' means it will do both. The default is 'combined'. In role other than
                        these three is invalid.

        -seedservers <servers>
                        Set the list of servers the node should contact, to join the cluster. This
                        should be comma-delimited list of servers, in the form host:port. This option
                        is REQUIRED.

For more about run flags, see 'influxd help run'.
`)
}
