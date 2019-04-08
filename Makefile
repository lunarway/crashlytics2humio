.DEFAULT: build

build:
	go build main.go

test:
	go test ./...

# curl a Crashlytics webhook to localhost with token=test
test_crashlytics_webhook:
	curl -d '{ \
		"event": "issue_impact_change", \
		"payload_type": "issue", \
			"payload": { \
			"display_id": 123 , \
			"title": "Issue Title" , \
			"method": "methodName of issue", \
			"impact_level": 2, \
			"crashes_count": 54, \
			"impacted_devices_count": 16, \
			"url": "http://crashlytics.com/full/url/to/issue" \
		} \
	}' localhost:8080/webhook?token=test
