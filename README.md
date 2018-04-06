# CF Org Memory report

This uses the functions exposed in [reportingFuncs](https://github.com/john-k-ge/reportingFuncs).  The idea is to take a 
more functional approach when handling the data pipeline.  This makes testing and reuse much simpler.

## Data Flow

The data flows in a series of overlapping stages:
1. Using the number of total pages, we generate the org page URLs.
1. The `orgPageUrlChan` go routines will pull a page of 50 orgs, and drop each org into `orgPageChan`.
1. The `orgPageChan` go routines take an org and find its memory utilization, then add it to `memUtilChan`.
1. The `memUtilChan` go routines take an org and determine its memory quota, then add it to `memQuotaChan`.
1. The `memQuotaChan` go routines take an org, find the list of org managers and add it to `orgManagerChan`.

The orgs in the `orgManagerChan` are now fully-populated, so we simply range over them to generate the report.
