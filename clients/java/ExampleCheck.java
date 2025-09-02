import java.io.IOException;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;

public class ExampleCheck {
    public static void main(String[] args) throws Exception {
        String url = System.getenv("KH_REPORTING_URL");
        String uuid = System.getenv("KH_RUN_UUID");

        if (url == null || url.isEmpty() || uuid == null || uuid.isEmpty()) {
            System.err.println("KH_REPORTING_URL and KH_RUN_UUID must be set");
            System.exit(1);
        }

        // Replace this with your own check logic. If the check passes, leave
        // ok as true. To report a failure, set ok to false and provide an
        // error message.
        boolean ok = true;
        String error = "";

        // Example: report a failure with an error message
        // ok = false;
        // error = "something went wrong";

        report(url, uuid, ok, error);
    }

    private static void report(String url, String uuid, boolean ok, String errorMessage) throws IOException {
        String payload = "{\"ok\":true,\"errors\":[]}";
        if (!ok) {
            payload = "{\"ok\":false,\"errors\":[\"" + escape(errorMessage) + "\"]}";
        }

        HttpURLConnection conn = (HttpURLConnection) new URL(url).openConnection();
        conn.setRequestMethod("POST");
        conn.setRequestProperty("Content-Type", "application/json");
        conn.setRequestProperty("kh-run-uuid", uuid);
        conn.setDoOutput(true);

        try (OutputStream os = conn.getOutputStream()) {
            os.write(payload.getBytes(StandardCharsets.UTF_8));
        }

        int code = conn.getResponseCode();
        if (code != 200) {
            // Kuberhealthy rejects malformed reports with HTTP 400. Treat any
            // non-200 status as a failure so the container exits quickly.
            throw new IOException("unexpected response code: " + code);
        }
    }

    private static String escape(String s) {
        return s.replace("\\", "\\\\").replace("\"", "\\\"");
    }
}
