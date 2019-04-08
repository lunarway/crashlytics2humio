# Crashlytics incidents to Humio

This project forwards Crashlytics incident notifications to Humio.

This is considered in *alpha* stage. APIs and usage will change.

[![Build Status](https://travis-ci.com/lunarway/crashlytics2humio.svg?branch=master)](https://travis-ci.com/lunarway/crashlytics2humio)

# Design

Crashlytics can trigger [webhooks on incident impact changes](https://docs.fabric.io/android/crashlytics/custom-web-hooks.html?web%20hooks#custom-web-hooks).
This server will setup an HTTP endpoint for these webhooks and push the data into Humio.

All payload fields from the webhook is pushed as fields in Humio.

```json
{
  "event": "issue_impact_change",
  "payload_type": "issue",
  "payload": {
    "display_id": 123 ,
    "title": "Issue Title" ,
    "method": "methodName of issue",
    "impact_level": 2,
    "crashes_count": 54,
    "impacted_devices_count": 16,
    "url": "http://crashlytics.com/full/url/to/issue"
  }
}
```

Will result in these fields in Humio.

```
display_id:             123 ,
title:                  "Issue Title" ,
method:                 "methodName of issue",
impact_level:           2,
crashes_count:          54,
impacted_devices_count: 16,
url:                    "http://crashlytics.com/full/url/to/issue"
```

# Usage

Start the application with the required flags and it will listen of webhooks on `:8080/webhook`.

```
$ crashlytics2humio -h
Usage of crashlytics2humio:
  -crashlytics-auth-token string
    	crashlytics webhook authentication token (required)
  -humio-ingest-token string
    	humio ingest token (required)
  -humio-url string
    	humio http api url eg. https://cloud.humio.com (required)
  -timeout duration
    	server request timeouts (default 10s)
```

## Crashlytics auth token

Crashlytics support query paramters in their webhook setup.
Set the `token` parameter to a generated token and start the application with this as well.
This will ensure to authenticated all incomminig webhooks.

Read more about setting up webhooks in the [Crashlytics docs](https://docs.fabric.io/android/crashlytics/custom-web-hooks.html).

## Humio ingest token

To ship incidents to Humio you need an ingest token.
Read the [Humio docs](https://docs.humio.com/sending-data-to-humio/ingest-tokens/) on setting one up.

The application pushes structured data to Humio so you do not need to setup a parser.
