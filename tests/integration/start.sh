#!/bin/bash

cd /obd && ./tracker_server --trackerConfigPath "/obd/conf.tracker.ini" &

sleep 1

cd /obd  && ./obdserver --configPath "/obd/conf.ini"