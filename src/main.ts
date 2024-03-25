import * as core from '@actions/core';
import * as dockerToolkit from '@docker/actions-toolkit';

export async function run(): Promise<void> {

  try {
    //get Compose File input
    const composeFile: string = core.getInput('compose-file');

    // Log the current timestamp, wait, then log the new timestamp
    core.debug(new Date().toTimeString())
    const composeSpec = compose.parseComposeFile(composeFile);
    compose.buildAllServices(composeSpec);
    core.debug(new Date().toTimeString())

    // Set outputs for other workflow steps to use
    core.setOutput('time', new Date().toTimeString())
  } catch (error) {
    core.error(`Action error: ${error}`);
  }
}
