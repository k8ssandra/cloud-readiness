#!/bin/sh
c1=TestK8cSmoke3379617300
c2=TestK8cSmoke2041637316
c3=TestK8cSmoke140217545

./delegate.sh $c1 &
./delegate.sh $c2 &
./delegate.sh $c3 &
