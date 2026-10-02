import { localAgentStateFromStatus } from "./service.js";

const INITIAL_STATE = Object.freeze({
  nodeId: "",
  agentId: null,
  status: "unknown",
  bitbrowserStatus: "unknown",
  mainUserId: "",
  operatingSystem: "",
  cpuArchitecture: "",
  agentVersion: "",
  currentTaskId: null,
  pendingResultCount: 0,
  lastEventName: null,
});

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function applyStatus(state, status) {
  return Object.assign(state, localAgentStateFromStatus(status));
}

export function createLocalAgentStore({ service }) {
  if (!service) {
    throw new TypeError("createLocalAgentStore requires a Local Agent service");
  }

  const state = { ...INITIAL_STATE };

  return {
    snapshot() {
      return clone(state);
    },
    async refresh() {
      return clone(applyStatus(state, await service.status()));
    },
    async start() {
      return clone(applyStatus(state, await service.start()));
    },
    async stop() {
      return clone(applyStatus(state, await service.stop()));
    },
    async applyEvent(event) {
      if (!event || event.event !== "status") {
        return clone(state);
      }
      state.lastEventName = event.event;
      return clone(applyStatus(state, event.data));
    },
  };
}
