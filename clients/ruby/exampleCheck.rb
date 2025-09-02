#!/usr/bin/env ruby

require 'json'
require 'net/http'
require 'uri'

# reporting_url fetches the Kuberhealthy reporting endpoint from the environment.
def reporting_url
  url = ENV['KH_REPORTING_URL']
  return url if url && !url.empty?
  raise 'KH_REPORTING_URL not set'
end

# run_uuid fetches the unique run identifier from the environment.
def run_uuid
  uuid = ENV['KH_RUN_UUID']
  return uuid if uuid && !uuid.empty?
  raise 'KH_RUN_UUID not set'
end

# post_status sends a status report back to Kuberhealthy.
def post_status(ok:, errors: [])
  uri = URI(reporting_url)
  req = Net::HTTP::Post.new(uri)
  req['Content-Type'] = 'application/json'
  req['kh-run-uuid'] = run_uuid
  req.body = JSON.generate({ ok: ok, errors: errors })
  Net::HTTP.start(uri.hostname, uri.port) { |http| http.request(req) }
end

# report_success tells Kuberhealthy the check passed.
def report_success
  post_status(ok: true, errors: [])
end

# report_failure tells Kuberhealthy the check failed with messages.
def report_failure(messages)
  post_status(ok: false, errors: messages)
end

begin
  # TODO: add check logic here. For now, always succeed.
  report_success
rescue StandardError => e
  report_failure([e.message])
end
