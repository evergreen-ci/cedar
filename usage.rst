=====
Usage
=====

General Functionality
=====================

This tool returns a JSON document with Evergreen providers and distros,
as well as Amazon EC2, EBS, and S3 data.

The purpose is to produce an aggregated single view into build and test automation spending, across hosting environments. This information can be used to target optimization efforts, provide feedback, and allow for informed planning to make the highest impact changes possible.

An example usage of this tool would be ``sink spend --start 2017-05-23T17:00 --duration 1h --config cost/testdata/spend_test.yml``


Flags
=====

 This tool has 3 flags: config, duration, and start.

Config Flag
-----------

 The input to this flag is a path to a yaml file. This flag cannot be omitted.

An example config file can be seen at https://github.com/evergreen-ci/sink/blob/master/cost/testdata/spend_test.yml.

The following fields can be included:


.. list-table:: **s3_info (Required)**
   :widths: 25 10 55
   :header-rows: 1

   * - Fields
     - Type
     - Description
   * - ``s3key_start``
     - string
     - Base string for the S3 key (on which -YYYY-MM.csv will be appended)
   * - ``s3bucket``
     - string
     - Name of the S3 bucket


.. list-table:: **evergreen_info (Required)**
    :widths: 25 10 55
    :header-rows: 1

    * - Fields
      - Type
      - Description
    * - ``evergreen_user``
      - string
      - Evergreen username
    * - ``evergreen_api_key``
      - string
      - Evergreen secret key
    * - ``root_url``
      - string
      - Root URL for the evergreen API routes


.. list-table:: **pricing (Required)**
    :widths: 25 10 70
    :header-rows: 1

    * - Fields
      - Type
      - Description
    * - ``gp2``
      - float
      - us-east price per GB-month for gp2 ebs volumes
    * - ``io1GB``
      - float
      - us-east price per GB-month for io1 ebs volumes
    * - ``io1IOPS``
      - float
      - us-east price per IOPS-month for io1 ebs volumes
    * - ``st1``
      - float
      - us-east price per GB-month for st1 ebs volumes
    * - ``sc1``
      - float
      - us-east price per GB-month for sc1 ebs volumes
    * - ``standard``
      - float
      - us-east price per GB-month and per 1 million I/O requests for magnetic volumes

Details about these prices can be found at https://aws.amazon.com/ebs/pricing/.

.. list-table:: **opts**
   :widths: 25 10 70
   :header-rows: 1

   * - Fields
     - Type
     - Description
   * - ``duration``
     - time.Duration
     - Duration of the report (if omitted, default is 1h)
   * - ``directory``
     - string
     - Directory to save output files (if omitted, printed to stdout)

Additionally, aws_accounts is a []string holding aws account names, (required).
Providers is a list of provider structs, which have a name (string) and cost (float).
The providers struct is used to pass in provider information besides AWS (ex: macstadium).


Duration Flag
--------------

The input to this flag is a duration string:

 A duration string is a possibly signed sequence of decimal numbers, each with optional fraction and a unit suffix, such as
 "300ms", "-1.5h" or "2h45m". Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

If this flag is omitted, the default is 1h.
If this flag is set, the duration in the config file is ignored.

Note that the duration has a 1 hour default because Amazon terminated instances can be deleted within the hour, and so a duration of longer than 1 hour is likely to have some missing information.

Start Flag
----------

 The input to this flag is a timestamp in UTC of the form YYYY-MM-DDTHH:MM .
 If this flag is omitted, the default is set to the current time, minus the duration.

Note that the default is current time minus duration because Amazon terminated instances can be deleted within the hour, and so a start time farther in the past is likely to have some missing information.


Output
======

 The output is a JSON document.

If a directory is given in the configuration file, the output will be stored in a file named after the generated time and duration.
For example, 2017-08-01 13:24:07.735655759 -0400 EDT_4h0m0s.txt.

If a directory is not given, then the output will be printed to stdout.
The form of the JSON document is:

.. list-table:: **Output**
   :widths: 25 10 70
   :header-rows: 1

   * - Fields
     - Type
     - Description
   * - ``Report``
     - Report
     - Stores time information for report
   * - ``Evergreen``
     - Evergreen
     - Stores Evergreen data
   * - ``Providers``
     - []Provider
     - Stores Provider information

.. list-table:: **Report**
   :widths: 25 10 70
   :header-rows: 1

   * - Fields
     - Type
     - Description
   * - ``Generated``
     - String
     - The time the report was generated (time.Time.String())
   * - ``Begin``
     - String
     - The start time for the report information (time.Time.String())
   * - ``End``
     - String
     - The end time for the report information (time.Time.String())

.. list-table:: **Evergreen**
   :widths: 25 10 70
   :header-rows: 1

   * - Fields
     - Type
     - Description
   * - ``Projects``
     - []Project
     - The list of Evergreen projects
   * - ``Distros``
     - []Distros
     - The list of Evergreen distros

.. list-table:: **Provider**
   :widths: 25 10 70
   :header-rows: 1

   * - Fields
     - Type
     - Description
   * - ``Name``
     - string
     - The name of the provider
   * - ``Accounts``
     - []Account
     - The list of Accounts for the provider, if applicable
   * - ``Cost``
     - float32
     - The overall cost for the provider, if applicable

.. list-table:: **Project**
   :widths: 25 10 70
   :header-rows: 1

   * - Fields
     - Type
     - Description
   * - ``Name``
     - string
     - The name of the Evergreen project
   * - ``Tasks``
     - []Task
     - The list of Evergreen tasks for the project

.. list-table:: **Distro**
   :widths: 25 10 70
   :header-rows: 1

   * - Fields
     - Type
     - Description
   * - ``Name``
     - string
     - The name of the Evergreen distro
   * - ``Provider``
     - string
     - The name of the corresponding provider
   * - ``InstanceType``
     - string
     - The type of the instance for the distro
   * - ``InstanceSeconds``
     - int64
     - The number of seconds the distro has been running

.. list-table:: **Account**
   :widths: 25 10 70
   :header-rows: 1

   * - Fields
     - Type
     - Description
   * - ``Name``
     - string
     - The name of the account
   * - ``Services``
     - []Service
     - The list of services we have for this account

.. list-table:: **Task**
   :widths: 25 10
   :header-rows: 1

   * - Fields
     - Type
   * - ``Githash``
     - string
   * - ``Name``
     - string
   * - ``Distro``
     - string
   * - ``BuildVariant``
     - string
   * - ``TaskSeconds``
     - int64

.. list-table:: **Item**
   :widths: 25 10 70
   :header-rows: 1

   * - Fields
     - Type
     - Description
   * - ``Name``
     - string
     - The instance name for this item (ex: "c3.4xlarge")
   * - ``ItemType``
     - string
     - The type of the item (ex: "spot")
   * - ``Launched``
     - int
     - The number of launched instances of this name/type
   * - ``Terminated``
     - int
     - The number of terminated instances of this name/type
   * - ``FixedPrice``
     - float32
     - The fixed price for this instance (only reserved EC2 instances)
   * - ``AvgPrice``
     - float32
     - The average price for this instance
   * - ``AvgUptime``
     - float32
     - The average uptime for this instance
   * - ``TotalHours``
     - int
     - The uptime for all the items combined


Amazon Specifics
================
 We are concerned with EC2 Instances (spot, reserved, and on-demand), EBS Volumes, and S3 cost.

In the config file, we pass in a slice of account names. Note that the name of this account **must** match the header of its credentials in the ~/.aws/credentials file, and that all account credentials should be in this file. The name of this account must also match the LinkedAccountName in the billing information in the cost spreadsheets (case insensitive).

EC2 Instances -- spot
---------------------
For Spot Instances, we used spot codes to make assumptions on whether to ignore instances, treat instances as Amazon-terminated or user-terminated (if Amazon terminates an instance, we do not pay for the partial hour).

EC2 spot instances have all Item fields except for FixedPrice.

We use the function DescribeSpotPriceHistory to calculate the prices for these instances within their actual time frames.

EC2 Instances -- on-demand
--------------------------
We assume an instance is On-Demand if the InstanceLifecycle field is nil (aka the instance is not spot or scheduled). Additionally, the Platform field will tell us if the instance is Windows or not Windows, but not whether it’s Linux or SUSE or RHEL. For now we assume if the instance is not Windows then it is Linux.

EC2 on-demand instances have all Item fields except for FixedPrice.

We use current on-demand pricing for our estimations, so reports with an older start time may have skewed price information. This information is parsed from https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/index.json.

EC2 Instances -- reserved
-------------------------
For Reserved Instances, if the pricing model is not All Upfront, we assume the costs are hourly. The fixed price for reserved instances is not divided to be specific to the report range, but the uptime is limited to the report range. The price for Reserved Instances is attached to the instance, so no further action is required.

If the instance is *All Upfront*, then only FixedPrice will be used. If the instance has *No Upfront* then only the AvgPrice (treated as hourly * uptime) will be used. *Partial Upfront* will populate both fields, so EC2 reserved instances potentially have all Item fields.

EBS Volumes
-------------
As detailed in **Output**, EBS pricing must be passed in, and these prices are then calculated as described here:
https://aws.amazon.com/ebs/pricing/.

For EBS Volumes, we only populate the itemType, launched, terminated, and avgPrice fields.

S3 Buckets
----------
For S3, we retrieve pricing information from a csv file in an S3 bucket. The key for this file should be
keyStart-YYYY-MM.csv (where keyStart is passed in the config file). The year and month are chosen to match the start time of the report, but not necessarily the end time.

We do not store items for the S3 instances, but rather add the costs for all S3 buckets under the given account.
We assume that these account names are stored in the **10th column** (recall we compare these names to the names given in the config file, case insensitive). We also assume that the service name is stored in the **13th column** (where we filter by AmazonS3), and the cost itself is stored in the **29th column** (i.e. the last column). If the spreadsheet format were to change, these columns would need to be updated in s3_price.go.


Organization
============

amazon
------
 This package contains methods and structs used for retrieving cost information from Amazon services.

Functions used for getting EC2 pages are located in client.go, as well as functions for getting EBS volumes.
Functions specifically dealing with EBS pricing, on-demand EC2 pricing, spot EC2 pricing, and S3 pricing are sorted into their own files withing this package. Note that the function in s3_price.go which retrieves prices is called directly from cost/spend.go, *not* from client.go.

evergreen
---------
 This package contains methods and structs used for retrieving project and distro information from Evergreen.

Functions which can call the API routes are in client.go, none of which are public. However distros.go and packages.go use those functions to deal with distro and package cost information, respectively, and public functions are available here.

cost
----
 This package contains functions that collects both Amazon and Evergreen cost information.

The structs for the report (detailed above in **Output**) are located in model.go. The structs for the configuration file (detailed above in the **Config** flag section) are located in config.go.
In evergreen_spend.go, the data structures returned from the functions in the evergreen package are reorganized into the relevant structs in the cost package.
In spend.go, the data structures returned from the functions in the amazon package are reorganized into the relevant structs in the cost package. The ``CreateReport`` function takes both the amazon and evergreen information, as well as additional provider information from the configuration file, and returns the ``Output`` struct. The ``Print`` method on ``*Output`` prints the report to either stdout, or to a file in the given directory. These functions are called directly in operations.
Additionally, a sample yaml file for the configuration file is available in testdata/spend_test.yml.

operations
----------
 The relevant file in this package is spend.go, where the tool itself is defined.

In this file, we verify that a configuration file exists, the EBS Pricing struct exists, S3Info is non-empty (specifically bucket and keyStart), EvergreenInfo is non-empty (specifically that it has a user, key, and rootURL), and that there is at least one account in the Accounts slice. Note that the specific validation functions are in their respective packages (amazon/evergreen).

sink.go
--------
 The configuration file information is stored in the appServicesCache in sink.go, as the spendConfig field.

The functions setSpendConfig and getSpendConfig keep the appServicesCache updated with the current configuration file, and retrieves the information from here.
