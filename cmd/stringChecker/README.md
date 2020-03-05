## stringChecker

The stringChecker parses through the content body from a specified URL searching for a specified string. This check reports a success upon receiving a bool value indicating if the string existed and reported to Kuberhealthy servers.

You can specify the URL to check with the `TARGET_URL` environment variable in the `.yaml` file.

You can specify the string to check with the `TARGET_STRING` environment variable in the `.yaml` file.

#### Example stringChecker Spec
```yaml
---
  apiVersion: comcast.github.io/v1
  kind: KuberhealthyCheck
  metadata:
    name: kh-string-checker
  spec:
    runInterval: 60s # The interval that Kuberhealthy will run your check on
    timeout: 2m # After this much time, Kuberhealthy will kill your check and consider it "failed"
    podSpec: # The exact pod spec that will run.  All normal pod spec is valid here.
      containers:
      - image: jdowni000/string-checker:v1.1.1 # The image of the check you just pushed
        imagePullPolicy: IfNotPresent # uses local image if present
        name: main
        env:
          - name: "TARGET_URL"
            value: "http://a39b2df774eb311eabbe902ee0ba4f44-2011198479.us-west-2.elb.amazonaws.com/?car=Roadster" # The URL that application will use to look for a specified string
          - name: "TARGET_STRING"
            value: "driving mile 0" # The string that will be used to parse through provided URL

```

#### How-to

 Make sure you are using the latest release of Kuberhealthy 2.0.0.

 Apply a `.yaml` file similar to the one shown above with ```kubectl apply -f```
