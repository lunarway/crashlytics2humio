# Crashlytics incidents to Humio

This project forwarads Crashlytics incident notifications to Humio.

This is considered in *alpha* stage. APIs and usage will change.

[![Build Status](https://travis-ci.com/lunarway/crashlytics2humio.svg?branch=master)](https://travis-ci.com/lunarway/crashlytics2humio)

# Design

Crashlytics can trigger [webhooks on incident impact changes](https://docs.fabric.io/android/crashlytics/custom-web-hooks.html?web%20hooks#custom-web-hooks).
This server will setup an HTTP endpoint for these webhooks and push the data into Humio.

# Usage

Start the application with the required flags and it will listen of webhooks on `:8080/webhook`.

```
$ crashlytics2humio -crashlytics-auth-token auth-token
2019/04/07 12:20:31 Listening on :8080
```
