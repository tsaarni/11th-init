#!/bin/sh

echo "Going to crash..."
kill -SEGV $$
