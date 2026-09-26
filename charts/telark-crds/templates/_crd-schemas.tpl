{{- define "dateFieldsSchema" -}}
creationDate:
  type: string
  description: ISO 8601 timestamp when the resource was created
  format: date-time
lastUpdateDate:
  type: string
  description: ISO 8601 timestamp when the resource was last updated
  format: date-time
{{- end -}}


{{- define "usageBlockSchema" -}}
type: object
properties:
  qos:
    type: string
  resources:
    type: object
    properties:
      totalCpu:
        type: string
      totalMemory:
        type: string
      usagePerInstance:
        type: array
        items:
          type: object
          properties:
            name:
              type: string
            containers:
              type: array
              items:
                type: object
                properties:
                  name:
                    type: string
                  cpu:
                    type: string
                  memory:
                    type: string
            totalCpu:
              type: string
            totalMemory:
              type: string
  available:
    type: boolean
  timestamp:
    type: string
{{- end }}
{{- define "conditionsSchema" -}}
type: array
x-kubernetes-list-type: map
x-kubernetes-list-map-keys:
  - type
items:
  type: object
  required:
    - type
    - status
  properties:
    type:
      type: string
      maxLength: 316
    status:
      type: string
      enum: ["True", "False", "Unknown"]
    reason:
      type: string
      maxLength: 1024
    message:
      type: string
      maxLength: 32768
    lastTransitionTime:
      type: string
      format: date-time
    observedGeneration:
      type: integer
      format: int64
      minimum: 0
{{- end -}}
