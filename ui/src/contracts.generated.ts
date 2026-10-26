// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Generated from api/v1alpha1 by ui/contractgen. Do not edit.

export type JsonValue = null | boolean | number | string | JsonValue[] | { [key: string]: JsonValue };
export interface Action {
  name: string;
  project: string;
  templateRef: string;
}
export interface AgentTemplate {
  apiVersion: string;
  kind: string;
  metadata: ObjectMeta;
  spec: AgentTemplateSpec;
}
export interface AgentTemplateArtifact {
  image: string;
}
export interface AgentTemplateCapabilityCeiling {
  maxTimeout?: (string | null);
  memoryScopes?: (Array<string> | null);
  modelProfiles?: (Array<string> | null);
  networkProfiles?: (Array<string> | null);
  resourceScopes?: (Array<string> | null);
  runtimeProfiles?: (Array<string> | null);
  tools?: (Array<string> | null);
}
export interface AgentTemplateDefaults {
  memoryScopes?: (Array<string> | null);
  modelProfile?: string;
}
export interface AgentTemplateEntrypoint {
  command: (Array<string> | null);
}
export interface AgentTemplateSpec {
  artifact: (AgentTemplateArtifact | null);
  capabilityCeiling: (AgentTemplateCapabilityCeiling | null);
  defaults?: AgentTemplateDefaults;
  entrypoint: (AgentTemplateEntrypoint | null);
}
export interface Change {
  effective: string;
  field: string;
  reasonCode: string;
  requested: string;
}
export type ClaimPhase = "Pending" | "Bound" | "Running" | "Succeeded" | "Failed" | "Expired";
export interface ClaimRequest {
  apiVersion: string;
  kind: string;
  metadata: ObjectMeta;
  spec: ClaimRequestSpec;
}
export interface ClaimRequestSpec {
  projectRef?: string;
  requestedAccess?: ClaimRequestedAccess;
  runtime: (ClaimRuntimeRequirements | null);
  task: (ClaimRequestTask | null);
  templateRef: string;
}
export interface ClaimRequestTask {
  input?: Record<string, JsonValue> | null;
  type: string;
}
export interface ClaimRequestedAccess {
  memoryScopes?: (Array<string> | null);
  modelProfile?: string;
  networkProfiles?: (Array<string> | null);
  resourceScopes?: (Array<string> | null);
  tools?: (Array<string> | null);
}
export interface ClaimRuntimeRequirements {
  profileRef: string;
  timeout: (string | null);
}
export interface Completion {
  reason?: string;
  status: string;
  target?: string;
}
export interface Decision {
  action: string;
  id: string;
  policyRef: PolicyReference;
  principalRef: string;
  reason: string;
  result: DecisionResult;
}
export type DecisionResult = "Allow" | "Deny" | "ApprovalRequired";
export interface Diagnosis {
  code: string;
  stage: string;
}
export interface EffectiveAuthority {
  id: string;
  memoryScopes?: (Array<string> | null);
  modelProfile?: string;
  networkProfiles?: (Array<string> | null);
  resourceScopes?: (Array<string> | null);
  runtime: EffectiveAuthorityRuntime;
  tools?: (Array<string> | null);
}
export interface EffectiveAuthorityRuntime {
  profileRef: string;
  timeout: string;
}
export interface Evidence {
  claimId?: string;
  decisionIds?: (Array<string> | null);
  modelInvocations: (Array<EvidenceModelInvocation> | null);
  requestRef: string;
  runtimeEvents: (Array<EvidenceRuntimeEvent> | null);
  toolInvocations: (Array<EvidenceToolInvocation> | null);
}
export interface EvidenceModelInvocation {
  modelName?: string;
}
export interface EvidenceRuntimeEvent {
  kind: string;
}
export interface EvidenceToolInvocation {
  toolName?: string;
}
export interface Fact {
  authorityChanges?: (Array<Change> | null);
  backendIdentity?: (SandboxClaimBackendIdentity | null);
  claimId?: string;
  decision?: (Decision | null);
  effectiveAuthority?: (EffectiveAuthority | null);
  id: string;
  invocationId?: string;
  kind: string;
  operation?: string;
  policyRef?: (PolicyReference | null);
  providerStatus?: string;
  reason?: string;
  reasonCode?: string;
  requestRef: string;
  result?: DecisionResult;
  resultRef?: string;
  sequence: number;
  target?: string;
  timestamp: string;
  truncated?: boolean;
}
export interface IssuedState {
  action: Action;
  claim?: (SandboxClaim | null);
  decision: Decision;
  effectiveAuthority?: (EffectiveAuthority | null);
  evidence: Evidence;
  policyRef: PolicyReference;
  principal: Principal;
  requestRef: string;
}
export interface ModelResult {
  inputTokens: number;
  invocationId: string;
  model: string;
  outputTokens: number;
  responseId?: string;
}
export interface ObjectMeta {
  name: string;
}
export interface Outcome {
  completion?: (Completion | null);
  failure?: string;
  model?: (ModelResult | null);
  status: string;
  text?: string;
}
export interface PolicyReference {
  id: string;
  version: string;
}
export interface Principal {
  authenticationContext: string;
  subject: string;
  team: string;
}
export interface Recovery {
  cleanup: string;
  code: string;
}
export interface SandboxClaim {
  authorityRef: string;
  backendIdentity?: (SandboxClaimBackendIdentity | null);
  id: string;
  phase: ClaimPhase;
  requestRef: string;
  templateRef: string;
}
export interface SandboxClaimBackendIdentity {
  allocationID?: string;
  backend: string;
  workerId: string;
}
export interface View {
  diagnosis?: (Diagnosis | null);
  facts: (Array<Fact> | null);
  outcome?: (Outcome | null);
  recovery?: (Recovery | null);
  request: (ClaimRequest | null);
  requestRef: string;
  state?: (IssuedState | null);
  traceId?: string;
  version: string;
}

export const shapes = {
  "Action": {
    "kind": "object",
    "fields": {
      "name": {
        "shape": {
          "kind": "string"
        }
      },
      "project": {
        "shape": {
          "kind": "string"
        }
      },
      "templateRef": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "AgentTemplate": {
    "kind": "object",
    "fields": {
      "apiVersion": {
        "shape": {
          "kind": "string"
        }
      },
      "kind": {
        "shape": {
          "kind": "string"
        }
      },
      "metadata": {
        "shape": {
          "kind": "ref",
          "ref": "ObjectMeta"
        }
      },
      "spec": {
        "shape": {
          "kind": "ref",
          "ref": "AgentTemplateSpec"
        }
      }
    }
  },
  "AgentTemplateArtifact": {
    "kind": "object",
    "fields": {
      "image": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "AgentTemplateCapabilityCeiling": {
    "kind": "object",
    "fields": {
      "maxTimeout": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "string"
          }
        },
        "optional": true
      },
      "memoryScopes": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "modelProfiles": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "networkProfiles": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "resourceScopes": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "runtimeProfiles": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "tools": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      }
    }
  },
  "AgentTemplateDefaults": {
    "kind": "object",
    "fields": {
      "memoryScopes": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "modelProfile": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      }
    }
  },
  "AgentTemplateEntrypoint": {
    "kind": "object",
    "fields": {
      "command": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        }
      }
    }
  },
  "AgentTemplateSpec": {
    "kind": "object",
    "fields": {
      "artifact": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "AgentTemplateArtifact"
          }
        }
      },
      "capabilityCeiling": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "AgentTemplateCapabilityCeiling"
          }
        }
      },
      "defaults": {
        "shape": {
          "kind": "ref",
          "ref": "AgentTemplateDefaults"
        },
        "optional": true
      },
      "entrypoint": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "AgentTemplateEntrypoint"
          }
        }
      }
    }
  },
  "Change": {
    "kind": "object",
    "fields": {
      "effective": {
        "shape": {
          "kind": "string"
        }
      },
      "field": {
        "shape": {
          "kind": "string"
        }
      },
      "reasonCode": {
        "shape": {
          "kind": "string"
        }
      },
      "requested": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "ClaimPhase": {
    "kind": "enum",
    "values": [
      "Pending",
      "Bound",
      "Running",
      "Succeeded",
      "Failed",
      "Expired"
    ]
  },
  "ClaimRequest": {
    "kind": "object",
    "fields": {
      "apiVersion": {
        "shape": {
          "kind": "string"
        }
      },
      "kind": {
        "shape": {
          "kind": "string"
        }
      },
      "metadata": {
        "shape": {
          "kind": "ref",
          "ref": "ObjectMeta"
        }
      },
      "spec": {
        "shape": {
          "kind": "ref",
          "ref": "ClaimRequestSpec"
        }
      }
    }
  },
  "ClaimRequestSpec": {
    "kind": "object",
    "fields": {
      "projectRef": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "requestedAccess": {
        "shape": {
          "kind": "ref",
          "ref": "ClaimRequestedAccess"
        },
        "optional": true
      },
      "runtime": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "ClaimRuntimeRequirements"
          }
        }
      },
      "task": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "ClaimRequestTask"
          }
        }
      },
      "templateRef": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "ClaimRequestTask": {
    "kind": "object",
    "fields": {
      "input": {
        "shape": {
          "kind": "json-map"
        },
        "optional": true
      },
      "type": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "ClaimRequestedAccess": {
    "kind": "object",
    "fields": {
      "memoryScopes": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "modelProfile": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "networkProfiles": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "resourceScopes": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "tools": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      }
    }
  },
  "ClaimRuntimeRequirements": {
    "kind": "object",
    "fields": {
      "profileRef": {
        "shape": {
          "kind": "string"
        }
      },
      "timeout": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "string"
          }
        }
      }
    }
  },
  "Completion": {
    "kind": "object",
    "fields": {
      "reason": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "status": {
        "shape": {
          "kind": "string"
        }
      },
      "target": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      }
    }
  },
  "Decision": {
    "kind": "object",
    "fields": {
      "action": {
        "shape": {
          "kind": "string"
        }
      },
      "id": {
        "shape": {
          "kind": "string"
        }
      },
      "policyRef": {
        "shape": {
          "kind": "ref",
          "ref": "PolicyReference"
        }
      },
      "principalRef": {
        "shape": {
          "kind": "string"
        }
      },
      "reason": {
        "shape": {
          "kind": "string"
        }
      },
      "result": {
        "shape": {
          "kind": "ref",
          "ref": "DecisionResult"
        }
      }
    }
  },
  "DecisionResult": {
    "kind": "enum",
    "values": [
      "Allow",
      "Deny",
      "ApprovalRequired"
    ]
  },
  "Diagnosis": {
    "kind": "object",
    "fields": {
      "code": {
        "shape": {
          "kind": "string"
        }
      },
      "stage": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "EffectiveAuthority": {
    "kind": "object",
    "fields": {
      "id": {
        "shape": {
          "kind": "string"
        }
      },
      "memoryScopes": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "modelProfile": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "networkProfiles": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "resourceScopes": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "runtime": {
        "shape": {
          "kind": "ref",
          "ref": "EffectiveAuthorityRuntime"
        }
      },
      "tools": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      }
    }
  },
  "EffectiveAuthorityRuntime": {
    "kind": "object",
    "fields": {
      "profileRef": {
        "shape": {
          "kind": "string"
        }
      },
      "timeout": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "Evidence": {
    "kind": "object",
    "fields": {
      "claimId": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "decisionIds": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "string"
            }
          }
        },
        "optional": true
      },
      "modelInvocations": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "ref",
              "ref": "EvidenceModelInvocation"
            }
          }
        }
      },
      "requestRef": {
        "shape": {
          "kind": "string"
        }
      },
      "runtimeEvents": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "ref",
              "ref": "EvidenceRuntimeEvent"
            }
          }
        }
      },
      "toolInvocations": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "ref",
              "ref": "EvidenceToolInvocation"
            }
          }
        }
      }
    }
  },
  "EvidenceModelInvocation": {
    "kind": "object",
    "fields": {
      "modelName": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      }
    }
  },
  "EvidenceRuntimeEvent": {
    "kind": "object",
    "fields": {
      "kind": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "EvidenceToolInvocation": {
    "kind": "object",
    "fields": {
      "toolName": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      }
    }
  },
  "Fact": {
    "kind": "object",
    "fields": {
      "authorityChanges": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "ref",
              "ref": "Change"
            }
          }
        },
        "optional": true
      },
      "backendIdentity": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "SandboxClaimBackendIdentity"
          }
        },
        "optional": true
      },
      "claimId": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "decision": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "Decision"
          }
        },
        "optional": true
      },
      "effectiveAuthority": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "EffectiveAuthority"
          }
        },
        "optional": true
      },
      "id": {
        "shape": {
          "kind": "string"
        }
      },
      "invocationId": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "kind": {
        "shape": {
          "kind": "string"
        }
      },
      "operation": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "policyRef": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "PolicyReference"
          }
        },
        "optional": true
      },
      "providerStatus": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "reason": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "reasonCode": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "requestRef": {
        "shape": {
          "kind": "string"
        }
      },
      "result": {
        "shape": {
          "kind": "ref",
          "ref": "DecisionResult"
        },
        "optional": true
      },
      "resultRef": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "sequence": {
        "shape": {
          "kind": "number"
        }
      },
      "target": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "timestamp": {
        "shape": {
          "kind": "string"
        }
      },
      "truncated": {
        "shape": {
          "kind": "boolean"
        },
        "optional": true
      }
    }
  },
  "IssuedState": {
    "kind": "object",
    "fields": {
      "action": {
        "shape": {
          "kind": "ref",
          "ref": "Action"
        }
      },
      "claim": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "SandboxClaim"
          }
        },
        "optional": true
      },
      "decision": {
        "shape": {
          "kind": "ref",
          "ref": "Decision"
        }
      },
      "effectiveAuthority": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "EffectiveAuthority"
          }
        },
        "optional": true
      },
      "evidence": {
        "shape": {
          "kind": "ref",
          "ref": "Evidence"
        }
      },
      "policyRef": {
        "shape": {
          "kind": "ref",
          "ref": "PolicyReference"
        }
      },
      "principal": {
        "shape": {
          "kind": "ref",
          "ref": "Principal"
        }
      },
      "requestRef": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "ModelResult": {
    "kind": "object",
    "fields": {
      "inputTokens": {
        "shape": {
          "kind": "number"
        }
      },
      "invocationId": {
        "shape": {
          "kind": "string"
        }
      },
      "model": {
        "shape": {
          "kind": "string"
        }
      },
      "outputTokens": {
        "shape": {
          "kind": "number"
        }
      },
      "responseId": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      }
    }
  },
  "ObjectMeta": {
    "kind": "object",
    "fields": {
      "name": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "Outcome": {
    "kind": "object",
    "fields": {
      "completion": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "Completion"
          }
        },
        "optional": true
      },
      "failure": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "model": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "ModelResult"
          }
        },
        "optional": true
      },
      "status": {
        "shape": {
          "kind": "string"
        }
      },
      "text": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      }
    }
  },
  "PolicyReference": {
    "kind": "object",
    "fields": {
      "id": {
        "shape": {
          "kind": "string"
        }
      },
      "version": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "Principal": {
    "kind": "object",
    "fields": {
      "authenticationContext": {
        "shape": {
          "kind": "string"
        }
      },
      "subject": {
        "shape": {
          "kind": "string"
        }
      },
      "team": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "Recovery": {
    "kind": "object",
    "fields": {
      "cleanup": {
        "shape": {
          "kind": "string"
        }
      },
      "code": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "SandboxClaim": {
    "kind": "object",
    "fields": {
      "authorityRef": {
        "shape": {
          "kind": "string"
        }
      },
      "backendIdentity": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "SandboxClaimBackendIdentity"
          }
        },
        "optional": true
      },
      "id": {
        "shape": {
          "kind": "string"
        }
      },
      "phase": {
        "shape": {
          "kind": "ref",
          "ref": "ClaimPhase"
        }
      },
      "requestRef": {
        "shape": {
          "kind": "string"
        }
      },
      "templateRef": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "SandboxClaimBackendIdentity": {
    "kind": "object",
    "fields": {
      "allocationID": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "backend": {
        "shape": {
          "kind": "string"
        }
      },
      "workerId": {
        "shape": {
          "kind": "string"
        }
      }
    }
  },
  "View": {
    "kind": "object",
    "fields": {
      "diagnosis": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "Diagnosis"
          }
        },
        "optional": true
      },
      "facts": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "array",
            "item": {
              "kind": "ref",
              "ref": "Fact"
            }
          }
        }
      },
      "outcome": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "Outcome"
          }
        },
        "optional": true
      },
      "recovery": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "Recovery"
          }
        },
        "optional": true
      },
      "request": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "ClaimRequest"
          }
        }
      },
      "requestRef": {
        "shape": {
          "kind": "string"
        }
      },
      "state": {
        "shape": {
          "kind": "nullable",
          "item": {
            "kind": "ref",
            "ref": "IssuedState"
          }
        },
        "optional": true
      },
      "traceId": {
        "shape": {
          "kind": "string"
        },
        "optional": true
      },
      "version": {
        "shape": {
          "kind": "string"
        }
      }
    }
  }
} as const;
