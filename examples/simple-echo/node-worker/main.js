const { connectNATS, startWorker } = require("cap-sdk-node");

const JOB_SUBJECT = "job.echo";
const WORKER_ID = "simple-echo-node-worker";

async function main() {
  const nc = await connectNATS({
    url: process.env.CAP_NATS_URL ?? "nats://127.0.0.1:4222",
    name: WORKER_ID,
  });
  try {
    await startWorker({
      nc,
      subject: JOB_SUBJECT,
      senderId: WORKER_ID,
      handler: async (req) => ({
        jobId: req.jobId,
        status: "JOB_STATUS_SUCCEEDED",
        resultPtr: `demo://result/${req.jobId}`,
        workerId: WORKER_ID,
      }),
    });
    await nc.flush();
    console.log(`CAP_SIMPLE_ECHO_READY worker=${WORKER_ID} subject=${JOB_SUBJECT}`);
  } catch (error) {
    await nc.drain().catch(() => undefined);
    throw error;
  }
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
});
