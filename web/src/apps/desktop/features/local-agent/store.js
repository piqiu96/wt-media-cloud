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
  state.nodeId = status.node_id;
  state.agentId = status.agent_id;
  state.status = status.status;
  state.bitbrowserStatus = status.bitbrowser_status;
  state.mainUserId = status.main_user_id;
  state.operatingSystem = status.operating_system;
  state.cpuArchitecture = status.cpu_architecture;
  state.agentVersion = status.agent_version;
  state.currentTaskId = status.current_task_id;
  state.pendingResultCount = status.pending_result_count;
  return state;
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
