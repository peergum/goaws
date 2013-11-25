goaws
=====

An easy tool in GO to list and connect to Amazon instances

It requires trulioo/conf which parses a configuration file in YAML

_Usage_

Usage is pretty straight-forward. _goaws_ alone on a line shows the following help:

usage: goaws COMMAND [ARGS]

commands
* list   : Show instances
* ssh    : SSH to given instance
* rename : Rename instances

options:
* REGION
 * -region: region to access (e.g. us-west)
          accessKey and secretKey must be in config

* DISPLAY
 * -id: show instance id
 * -vpcid: show VPC id
 * -v: show VPC instances only
 * -e: show non-VPC instances only
 * -i: show IP or DNS name only

* FILTERS
 * -image xxx : only instances with image xxx (e.g.: ami-xxxxxx)
 * -type xxx  : only instances of type *xxx* (e.g.: small)
 * -state xxx : only instances in state *xxx* (e.g.: run)
 * -name xxx : only instances with name *xxx*
 * -stage xxx : only instances with tag "stage" as *xxx* (e.g.: prod)
