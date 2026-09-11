// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Generated from api/v1alpha1 by ui/contractgen. Do not edit.

export type JsonValue = null | boolean | number | string | JsonValue[] | { [key: string]: JsonValue };
export interface Action {
  name: string;
  project: string;
  templateRef: string;
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
  resourceScopes?: (Array<string> | null);
  tools?: (Array<string> | null);
}
export interface ClaimRuntimeRequirements {
  profileRef: string;
  timeout: (string | null);
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
export interface EffectiveAuthority {
  id: string;
  memoryScopes?: (Array<string> | null);
  modelProfile?: string;
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
export interface ObjectMeta {
  name: string;
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
export interface SandboxClaim {
  authorityRef: string;
  backendIdentity?: (SandboxClaimBackendIdentity | null);
  id: string;
  phase: ClaimPhase;
  requestRef: string;
  templateRef: string;
}
export interface SandboxClaimBackendIdentity {
  backend: string;
  workerId: string;
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
  }
} as const;
