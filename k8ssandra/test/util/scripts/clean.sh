#!/bin/sh
c1=TestK8cSmoke301164804
c2=TestK8cSmoke1789200748
c3=TestK8cSmoke1765091164

./delegate.sh $c1 &
./delegate.sh $c2 &
./delegate.sh $c3 &
