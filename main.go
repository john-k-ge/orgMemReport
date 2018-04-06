package main

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/john-k-ge/reportingFuncs/entity"
	"github.com/john-k-ge/reportingFuncs/funcs"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Println("You must provide a datacenter arg:")
		fmt.Println("us-e, us-w, jp, ff, az, or cf3")
		os.Exit(0)
	}
	dc := args[0]

	helper, err := funcs.ReportingHelperFactory(dc)
	if err != nil {
		fmt.Printf("Unrecognized datacenter: %v\n", dc)
		fmt.Println("You must provide a datacenter arg:")
		fmt.Println("us-e, us-w, jp, ff, az, or cf3")
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}

	err = helper.Authenticate()
	if err != nil {
		log.Printf("Failed to authenticate: %v", err)
		os.Exit(1)
	}

	fileName := "mem_util_" + time.Now().Format(time.RFC3339) + "_" + strings.ToUpper(dc) + ".txt"
	outputFile, err := os.Create(fileName)
	if err != nil {
		panic("Could not create output file: " + err.Error())
	}

	defer outputFile.Close()

	pageCount, err := helper.GetPageCount()
	if err != nil {
		log.Printf("Failed to get page count: %v", err)
		panic("Failed to get page count: " + err.Error())
	}
	log.Printf("PageCount: %v\n", pageCount)

	//Build the functions:
	buildOrgPageUrls := helper.GenOrgUrlListF()
	getOrgPage := helper.GenOrgPageF()
	getOrgUsers := helper.GenOrgUserF()
	getMemQuota := helper.GenMemQuotaF()
	getMemUtil := helper.GenMemUtilF()

	var orgPageUrlChan = make(chan *url.URL, pageCount)
	var orgPageChan = make(chan *entity.OrgInfo, pageCount)
	var memUtilChan = make(chan *entity.OrgInfo, pageCount*50)
	var memQuotaChan = make(chan *entity.OrgInfo, pageCount*50)
	var orgManagerChan = make(chan *entity.OrgInfo, pageCount*50)

	var orgPageWG, memUtilWG, memQuotaWG, orgManagerWG sync.WaitGroup

	for _, url := range buildOrgPageUrls(pageCount) {
		orgPageUrlChan <- url
	}

	close(orgPageUrlChan)

	log.Print("Starting org page fetch...\n")
	for i := 1; i <= 12; i++ {
		orgPageWG.Add(1)
		go func() {
			defer orgPageWG.Done()
			for pageUrl := range orgPageUrlChan {
				res := getOrgPage(pageUrl)
				for _, org := range res {
					orgPageChan <- org
				}
			}
		}()
		//go getOrgPage(orgPageUrlChan, orgPageChan)
	}

	log.Print("Starting concurrent mem util check...\n")
	for j := 1; j <= 3; j++ {
		memUtilWG.Add(1)
		go func() {
			defer memUtilWG.Done()
			for org := range orgPageChan {
				org.Mem_util = getMemUtil(org.Guid)
				memUtilChan <- org
			}
		}()
		//go getMemUtil(orgPageChan, memUtilChan)
	}

	log.Print("Starting concurrent memory quota check...\n")
	for k := 1; k <= 3; k++ {
		memQuotaWG.Add(1)
		go func() {
			defer memQuotaWG.Done()
			for org := range memUtilChan {
				org.Quota_name, org.Mem_quota = getMemQuota(org)
				memQuotaChan <- org
			}
		}()
		//go getMemQuota(memUtilChan, memQuotaChan)
	}

	log.Print("Starting concurrent Org Manager query...\n")
	for l := 1; l <= 25; l++ {
		orgManagerWG.Add(1)
		go func() {
			defer orgManagerWG.Done()
			for org := range memQuotaChan {
				org.Managers = getOrgUsers(org.Managers_url)
				orgManagerChan <- org
			}
		}()
		//go getOrgUsers(memQuotaChan, orgManagerChan)
	}

	log.Printf("I'm processing approximately %v records... Please be patient\n", pageCount*50)

	orgPageWG.Wait()
	log.Print("Got all orgs...\n")
	close(orgPageChan)

	memUtilWG.Wait()
	log.Print("Calculated memory util for each...\n")
	close(memUtilChan)

	memQuotaWG.Wait()
	log.Print("Found quotas for each org...\n")
	close(memQuotaChan)

	orgManagerWG.Wait()
	log.Print("Found org managers...\n")
	close(orgManagerChan)

	writeBuff := bufio.NewWriter(outputFile)
	writeBuff.WriteString("OrgGUID,OrgName,Created,QuotaName,MemUtil,MemQuota,OrgStatus,OrgManagers\n")
	writeBuff.Flush()

	for m := range orgManagerChan {
		writeBuff.WriteString(strings.Join([]string{m.Guid, m.Name, m.Created, m.Quota_name, strconv.Itoa(m.Mem_util),
			strconv.Itoa(m.Mem_quota), m.Status}, ","))
		for _, manager := range m.Managers {
			if len(manager) > 0 {
				writeBuff.WriteString("," + manager)
			}
		}

		writeBuff.WriteString("\n")
		writeBuff.Flush()
	}

	log.Println("All done...")
	log.Printf("Output written to: %v\n", fileName)
}
