package main

/*
#cgo pkg-config: ubuntu-app-launch-2
#include <stdio.h>
#include <glib.h>

extern void go_observer();
void stop_observer (const gchar * appid, const gchar * instanceid, const gchar * helpertype, gpointer user_data) {
    printf("%s | %s | %s \n", appid, instanceid, helpertype);
    go_observer();
}

*/
import "C"

