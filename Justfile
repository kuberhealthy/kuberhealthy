# https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/generating-clientset.md
# Generate API go files
generate:
	cd hack && ./update-codegen.sh
