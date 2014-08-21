#!/bin/sh
for a in `seq 1 100`
do
echo BOOM-$a
>&2 echo BANG-$a
done
exit 1
