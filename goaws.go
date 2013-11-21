package main

import (
	"flag"
	"fmt"
	aws "github.com/mitchellh/goamz/aws"
	ec2 "github.com/mitchellh/goamz/ec2"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"trulioo/conf"
)

var (
	Auth                 aws.Auth
	EC2                  *ec2.EC2
	showId               *bool   = flag.Bool("id", false, "Show Instance Id")
	showVpcId            *bool   = flag.Bool("vpcid", false, "Show VPC Id")
	vpcOnly              *bool   = flag.Bool("v", false, "Show VPC Intances only")
	ec2Only              *bool   = flag.Bool("e", false, "Show EC2 Intances only")
	ipOnly               *bool   = flag.Bool("i", false, "Show IP or DNS name only")
	region               *string = flag.String("r", "us-west-2", "Define region (us-west,us-west-2... [see aws.Regions]")
	progName                     = filepath.Base(os.Args[0])
	config               *conf.YamlConf
	accessKey, secretKey string
)

func init() {
	config = conf.GetConf()
	credentials, err := config.GetStringsMap("")
	//fmt.Println(regionKeys, err)

	if err != nil {
		fmt.Println("No credentials defined in config")
		os.Exit(2)
	}
	accessKey = credentials["accessKey"]
	secretKey = credentials["secretKey"]
}

func usage() {
	fmt.Println(`
usage: ` + progName + ` COMMAND [ARGS]

commands
 list   : Show instances
 ssh    : SSH to given instance
 rename : Rename instances

options:
 REGION
 -region: region to access (e.g. us-west)
          accessKey and secretKey must be in config
 DISPLAY
 -id: show instance id
 -vpcid: show VPC id
 -v: show VPC instances only
 -e: show non-VPC instances only
 -i: show IP or DNS name only
 FILTERS
 -image xxx : only instances with image xxx (e.g.: ami-xxxxxx)
 -type xxx  : only instances of type *xxx* (e.g.: small)
 -state xxx : only instances in state *xxx* (e.g.: run)
 -name xxx : only instances with name *xxx*
 -stage xxx : only instances with tag "stage" as *xxx* (e.g.: prod)
        `)
	os.Exit(255)
}

func main() {
	flag.Parse()

	args := flag.Args()
	//fmt.Println(args)
	if len(args) < 1 {
		usage()
	}
	Auth = aws.Auth{accessKey, secretKey, ""}
	awsRegion := aws.Regions[*region]
	EC2 = ec2.New(Auth, awsRegion)

	switch args[0] {
	case "list":
		args = args[1:]
		filter, args := getfilter(args)
		if len(args) > 0 {
			filter.Add("tag:Name", "*"+args[0]+"*")
		}
		instances, err := EC2.Instances(nil, filter)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		list(instances)
	case "rename":
		args = args[1:]
		filter, args := getfilter(args)
		if len(args) > 1 {
			filter.Add("tag:Name", "*"+args[0]+"*")
			args = args[1:]
		}
		if len(args) == 0 {
			fmt.Println("Missing new name")
			usage()
		}
		name := args[0]
		instances, err := EC2.Instances(nil, filter)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		nInst := list(instances)
		var confirm string = ""
		for confirm != "y" && confirm != "Y" && confirm != "n" && confirm != "N" {
			fmt.Println("\nAre you sure you want to rename these", nInst, "instances to", name+"_xx", "[y,N] ? ")
			fmt.Scanf("%s", &confirm)
		}
		if confirm == "y" || confirm == "Y" {
			fmt.Println("OK!")
		}
		rename(instances, name, nInst)
	case "stop":
		fmt.Println("stop")
	case "ssh":
		args = args[1:]
		if len(args) < 1 {
			fmt.Println("Missing host")
			usage()
		}
		filter, args := getfilter(args)
		if len(args) > 0 {
			filter.Add("tag:Name", "*"+args[0]+"*")
		}
		instances, err := EC2.Instances(nil, filter)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		ssh(instances)
	default:
		filter := ec2.NewFilter()
		filter.Add("tag:Name", "*"+args[0]+"*")
		instances, err := EC2.Instances(nil, filter)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		list(instances)
	}
}

func getfilter(args []string) (*ec2.Filter, []string) {
	filter := ec2.NewFilter()
	for len(args) > 1 {
		if args[0][0] != '-' {
			break
		}
		switch args[0] {
		case "-image":
			filter.Add("image-id", args[1])
		case "-type":
			filter.Add("instance-type", "*"+args[1]+"*")
		case "-state":
			filter.Add("instance-state-name", "*"+args[1]+"*")
		case "-name":
			filter.Add("tag:Name", "*"+args[1]+"*")
		case "-stage":
			filter.Add("tag:stage", "*"+args[1]+"*")
		default:
			filter.Add(args[0][1:], "*"+args[1]+"*")
		}
		if len(args) > 2 {
			args = args[2:]
		} else {
			args = nil
			break
		}
		//filter.Add("tag:Name", "*"+args[1]+"*")
	}
	return filter, args
}

func rename(instances *ec2.InstancesResp, name string, total int) {
	iNum := 1
	var digits string
	if total > 0 {
		digits = strconv.Itoa(int(math.Ceil(math.Log10(float64(total)))))
	}
	for _, reservation := range instances.Reservations {
		for _, instance := range reservation.Instances {
			instId := instance.InstanceId
			tag := ec2.Tag{
				Key:   "Name",
				Value: name + "_" + fmt.Sprintf("%0"+digits+"d", iNum),
			}
			fmt.Println("Name:", tag.Value)
			EC2.CreateTags([]string{instId}, []ec2.Tag{tag})
			iNum++
		}
	}
}

func ssh(instances *ec2.InstancesResp) {
	found := 0
	list := make([]string, 0)
	var name, address string
	for _, reservation := range instances.Reservations {
		for _, instance := range reservation.Instances {
			vpc := instance.VpcId
			name = "UNNAMED"
			for _, tag := range instance.Tags {
				if tag.Key == "Name" {
					name = tag.Value
				}
			}

			// print name
			dnsName := instance.DNSName
			if dnsName == "" {
				dnsName = "-"
			}
			ipAddress := instance.PrivateIpAddress
			if ipAddress == "" {
				ipAddress = "-"
			}
			if vpc != "" {
				address = ipAddress
			} else {
				address = dnsName
			}
			list = append(list, name+": "+address)
			found++
		}
	}
	if found > 1 {
		fmt.Println("Too many matches. Be more specific:")
		for _, item := range list {
			fmt.Println(item)
		}
		os.Exit(3)
	}
	fmt.Println("Launching SSH to", name, "at", address)
	sshCommand, err := exec.LookPath("ssh")
	if err != nil {
		fmt.Println("Couldn't find SSH...")
		os.Exit(5)
	}
	terminal, _ := syscall.Getenv("TERM")
	fmt.Println("Terminal:", terminal)
	//cmd := exec.Command("setterm", "-term "+terminal)
	//cmd.Run()
	syscall.Exec(sshCommand, []string{"ssh", address}, []string{"TERM=" + terminal})
}

func list(instances *ec2.InstancesResp) (numInstances int) {
	for _, reservation := range instances.Reservations {
		for _, instance := range reservation.Instances {
			numInstances++
			name := "UNNAMED"
			for _, tag := range instance.Tags {
				if tag.Key == "Name" {
					name = tag.Value
				}
			}

			vpc := instance.VpcId
			if *vpcOnly && vpc == "" {
				continue
			}
			if *ec2Only && vpc != "" {
				continue
			}

			// print name
			if !*ipOnly {
				fmt.Printf("%-20s", name)
			}

			dnsName := instance.DNSName
			if dnsName == "" {
				dnsName = "-"
			}
			ipAddress := instance.PrivateIpAddress
			if ipAddress == "" {
				ipAddress = "-"
			}

			if !*ipOnly && !*ec2Only && !*vpcOnly {
				// print VPC/EC2 type
				if vpc == "" {
					fmt.Printf(" EC2")
				} else {
					fmt.Printf(" VPC")
				}
			}

			// print instance Id
			if *showId {
				fmt.Printf(" %-10s", instance.InstanceId)
			}
			//print VPC Id
			if *showVpcId {
				fmt.Printf(" %-10s", instance.VpcId)
			}

			if !*ipOnly {
				fmt.Printf(" ")
			}
			// print address
			if vpc == "" {
				fmt.Printf("%s", dnsName)
			} else {
				fmt.Printf("%s", ipAddress)
			}
			fmt.Printf("\n")
		}
	}
	return numInstances
}
