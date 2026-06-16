// Veeam backup agent job status mapping to numeric values for Prometheus
function backupAgentJobStatus(hostStatus) {
    var val = "0";
    switch (hostStatus) {
        case "Online":
            val = "1";
            break;
        case "Offline":
            val = "2";
            break;
    }
    return val;
}

// Veeam version mapping to human readable format
function veeamVersion(version) {
    var vers = version;
    switch (vers) {
        case "12.3.2.4465":
        case "12.3.2.3617":
            vers = "12.3.2"; break;

        case "12.3.1.1139":
            vers = "12.3.1"; break;
        case "12.3.0.310":
            vers = "12.3.0"; break;

        case "12.2.0.334":
            vers     = "12.2.0"; break;
        case "12.1.2.172":
            vers = "12.1.2"; break;
        case "12.1.1.56":
            vers = "12.1.1"; break;
        case "12.1.0.2131":
            vers = "12.1.0"; break;
        case "12.0.0.1420":
            vers = "12.0 GA"; break;
        case "12.0.0.1402":
            vers = "12.0 RTM"; break;
        case "11.0.0.837":
            vers = "11.0 GA"; break;
        case "11.0.0.825":
            vers = "11.0 RTM"; break;
        case "10.0.1.4854":
            vers = "10.0a GA"; break;
        case "10.0.1.4848":
            vers = "10.0a RTM"; break;
        case "10.0.0.4461":
            vers = "10.0 GA"; break;
        case "10.0.0.4442":
            vers = "10.0 RTM"; break;
        case "9.5.4.2866":
            vers = "9.5 U4b GA"; break;
        case "9.5.4.2753":
            vers = "9.5 U4a GA"; break;
        case "9.5.4.2615":
            vers = "9.5 U4 GA"; break;
        case "9.5.4.2399":
            vers = "9.5 U4 RTM"; break;
        case "9.5.0.1922":
            vers = "9.5 U3a"; break;
        case "9.5.0.1536":
            vers = "9.5 U3"; break;
        case "9.5.0.1038":
            vers = "9.5 U2"; break;
        case "9.5.0.823":
            vers = "9.5 U1"; break;
        case "9.5.0.802":
            vers = "9.5 U1 RC"; break;
        case "9.5.0.711":
            vers = "9.5 GA"; break;
        case "9.5.0.580":
            vers = "9.5 RTM"; break;
        case "9.0.0.1715":
            vers = "9.0 U2"; break;
        case "9.0.0.1491":
            vers = "9.0 U1"; break;
        case "9.0.0.902":
            vers = "9.0 GA"; break;
        case "9.0.0.773":
            vers = "9.0 RTM"; break;
        case "8.0.0.2084":
            vers = "8.0 U3"; break;
        case "8.0.0.2030":
            vers = "8.0 U2b"; break;
        case "8.0.0.2029":
            vers = "8.0 U2a"; break;
        case "8.0.0.2021":
            vers = "8.0 U2 GA"; break;
        case "8.0.0.2018":
            vers = "8.0 U2 RTM"; break;
        case "8.0.0.917":
            vers = "8.0 P1"; break;
        case "8.0.0.817":
            vers = "8.0 GA"; break;
        case "8.0.0.807":
            vers = "8.0 RTM"; break;
    }
    return vers;
}

function currentElementStatus(job) {
    if( job.result === undefined || job.result === "None" ) {
        return job.state;
    } else {
        return job.result;
    }
}
// Veeam job status mapping to numeric values for Prometheus
//  Possible Result value: Success / Warning / Failed
//  Possible State value: Starting / Stopping / Working / Pausing / Resuming / Stopped  
function jobStatus(status) {
    var val = "0";
    switch (status) {
        case "Unknown":
            val = "0";
            break;
        case "Success":
            val = "1";
            break;
        case "Warning":
            val = "2";
            break;
        case "Failed":
            val = "3";
            break;
        case "Idle":
            val = "4";
            break;
        case "Working":
            val = "5";
            break;
        case "Starting":
            val = "6";
            break;
        case "Stoping":
            val = "7";
            break;
        case "Pausing":
            val = "8";
            break;
        case "Resuming":
            val = "9";
            break;
        case "Stopped":
            val = "10";
            break;
    }
    return val;
}

function getTimestamp(date_str) {
    if ( date_str[date_str.length-1] == 'Z') {
        date_str = date_str.slice(0, -1);
    }

    dt = new Date(date_str)
    return Math.floor( dt.getTime() / 1000 );
}

function buildJobs(jobs, raw_job) {

    var cur_end_timestamp = 0;
    var hash = exporter.sha1sum( raw_job.backupserver + raw_job.jobName + raw_job.jobType );
//    var job_start = exporter.toDate("2006-01-02T15:04:05Z", job.creationTimeUTC);
    var job_start = getTimestamp(raw_job.creationTimeUTC);
    var current_state = currentElementStatus( raw_job );
    current_state = jobStatus( current_state );

    if( raw_job.endTimeUTC !== undefined ) {
        // cur_end_timestamp = exporter.toDate("2006-01-02T15:04:05Z", job.endTimeUTC);
        cur_end_timestamp =  getTimestamp(raw_job.endTimeUTC);
    } else {
        cur_end_timestamp = Math.floor( Date.now() / 1000 );
    }

    // check if current job''s hash is already in job''s map
    if (jobs[hash] === undefined) {

        jobs[hash] = {
            "backupserver": raw_job.backupserver,
            "jobname": raw_job.jobName,
            "name": raw_job.name,
            "jobtype": raw_job.jobType,
            "state": current_state,
            "starttimestamp": job_start,
            "progress": raw_job.progress,
            "endtimestamp": cur_end_timestamp,
            "uid": raw_job.uid.split(":").slice(-1)[0],
            "retries": 0
        }
    } else {
      // old_job is a job set previously and is map[string]any
      var job = jobs[hash];
      var retries = ( job.retries === undefined ? 0 : job.retries ) + 1;
      if ( job.starttimestamp === undefined || job.starttimestamp < job_start ) {
        job.state = current_state;
        job.starttimestamp = job_start;
        job.endtimestamp = cur_end_timestamp;
        job.progress = raw_job.progress;
        job.uid = raw_job.uid.split(":").slice(-1)[0];
        job.retries = retries;
      } else {
        job.retries = retries;
      }
    }
    return job;
}

function taskStatus(status) {
    var val = "0";
    switch (status) {
        case "Unknown":
            val = "0";
            break;
        case "Success":
            val = "1";
            break;
        case "Warning":
            val = "2";
            break;
        case "Failed":
            val = "3";
            break;
        case "Pending":
            val = "4";
            break;
        case "InProgress":
            val = "5";
            break;
    }
    return val;
}
function findLinkElementByType(item, type) {
    var links = item.links
        for (var i = 0; i < links.length; i++) {
        if (links[i][item.typeName] === type) {
            return links[i];
        }
    }
    return null;
}

function buildTasks(tasks, raw_task) {

    var cur_end_timestamp = 0;
    var hash = exporter.sha1sum( raw_task.backupServerName + raw_task.jobName + raw_task.vmDisplayName );
    // var task_start = exporter.toDate("2006-01-02T15:04:05Z", item.CreationTimeUTC);
    var task_start = getTimestamp(raw_task.creationTimeUTC);
    var current_state = currentElementStatus( raw_task );
    current_state = taskStatus( current_state );

    if( raw_task.endTimeUTC !== undefined ) {
        // cur_end_timestamp = exporter.toDate("2006-01-02T15:04:05Z", item.EndTimeUTC);
        cur_end_timestamp =  getTimestamp(raw_task.endTimeUTC);
    } else {
        cur_end_timestamp = Math.floor( Date.now() / 1000 );
    }

    // check if current task's hash is already in task's map
    if (tasks[hash] === undefined) {
        var backupserver = findLinkElementByType(raw_task, "BackupServerReference");
        var jobname = findLinkElementByType(raw_task, "BackupJobSessionReference");
        tasks[hash] = {
            "backupserver": backupserver ? backupserver[raw_task.nameName] : "no_value",
            "jobname": jobname ? jobname[raw_task.nameName] : "no_value",
            "taskname": raw_task.name,
            "vmname": raw_task.vmDisplayName,
            "state": current_state,
            "starttimestamp": task_start,
            "endtimestamp": cur_end_timestamp,
            "uid": raw_task.uid.split(":").slice(-1)[0],
            "total_size_bytes": raw_task.totalSize,
            "reason": raw_task.reason,
            "retries": 0
        }
    } else {
      // old_task is a task set previously and is obj
      var task = tasks[hash];
      var retries = ( task.retries === undefined ? 0 : task.retries ) + 1;
      if ( task.starttimestamp === undefined || task.starttimestamp < task_start ) {
        task.state = current_state;
        task.starttimestamp = task_start;
        task.endtimestamp = cur_end_timestamp;
        task.uid = raw_task.uid.split(":").slice(-1)[0];
        task.total_size_bytes = raw_task.totalSize;
        task.reason = raw_task.reason;
        task.retries = retries;
      } else {
        task.retries = retries;
      }
    }
    return task;
}

// name for collector veeam_overview_metrics (veeam_overview_metrics.collector.yml)
function veeamNames(version) {
    var names;
    if(version >= 13) {
        names = {
            backupServers: 'backupServers',
            proxyServers: 'proxyServers',
            repositoryServers: 'repositoryServers',
            scheduledJobs: 'scheduledJobs',
            successfulVmLastestStates: 'successfulVmLastestStates',
            warningVmLastestStates: 'warningVmLastestStates',
            failedVmLastestStates: 'failedVmLastestStates',
        }
    } else {
        names = {
            backupServers: 'BackupServers',
            proxyServers: 'ProxyServers',
            repositoryServers: 'RepositoryServers',
            scheduledJobs: 'ScheduledJobs',
            successfulVmLastestStates: 'SuccessfulVmLastestStates',
            warningVmLastestStates: 'WarningVmLastestStates',
            failedVmLastestStates: 'FailedVmLastestStates',
        }
    }
    return names;
}

// name for collector veeam_overview_vms_metrics (veeam_overview_vms_metrics.collector.yml)
function vmNames(version) {
    var names;
    if(version >= 13) {
        names = {
            protectedVms: 'protectedVms',
            backedUpVms: 'backedUpVms',
            replicatedVms: 'replicatedVms',
            restorePoints: 'restorePoints',
            fullBackupPointsSize: 'fullBackupPointsSize',
            incrementalBackupPointsSize: 'incrementalBackupPointsSize',
            replicaRestorePointsSize: 'replicaRestorePointsSize',
            sourceVmsSize: 'sourceVmsSize',
            successBackupPercents: 'successBackupPercents',
        }
    } else {
        names = {
            protectedVms: 'ProtectedVms',
            backedUpVms: 'BackedUpVms',
            replicatedVms: 'ReplicatedVms',
            restorePoints: 'RestorePoints',
            fullBackupPointsSize: 'FullBackupPointsSize',
            incrementalBackupPointsSize: 'IncrementalBackupPointsSize',
            replicaRestorePointsSize: 'ReplicaRestorePointsSize',
            sourceVmsSize: 'SourceVmsSize',
            successBackupPercents: 'SuccessBackupPercents',
        }
    }
    return names;
}

// names for collector veeam_agent_metrics (veeam_agent_metrics.collector.yml)
function agentsNames(version) {
    var names;
    if(version >= 13) {
        names = {
            uid: 'uid',
            name: 'name',
            version: 'agentVersion',
            osVersion: 'osVersion',
            hostStatus: 'hostStatus',
            discoveredComputers: 'discoveredComputers',
            backupServer: 'toto'
        };
    } else {
        names = {
            uid: 'UID',
            name: 'Name',
            version: 'AgentVersion',
            osVersion: 'OsVersion',
            hostStatus: 'HostStatus',
            discoveredComputers: 'DiscoveredComputers',
            backupServer: 'toto'
        };
    }
    return names
}

// names for collector veeam_repositories_overview_metrics (veeam_repositories_overview_metrics.collector.yml)
function repositoriesNames(version) {
    var names;
    if(version >= 13) {
        names = {
            repositories: 'repositories',
            name: 'name',
            kind: 'kind',
            uid: 'uid',
            capacity: 'capacity',
            freeSpace: 'freeSpace',
        };
    } else {
        names = {
            repositories: 'Repositories',
            name: 'Name',
            kind: 'Kind',
            uid: 'UID',
            capacity: 'Capacity',
            freeSpace: 'FreeSpace',
        }
    }
    return names
}

// names for collector veeam_job_overview_metrics (veeam_job_overview_metrics.collector.yml)
function jobStatisticsNames(version) {
    var names;
    if(version >= 13) {
        names = {
            runningJobs: 'runningJobs',
            scheduledJobs: 'scheduledJobs',
            scheduledBackupJobs: 'scheduledBackupJobs',
            scheduledReplicaJobs: 'scheduledReplicaJobs',
            totalJobRuns: 'totalJobRuns',
            successfulJobRuns: 'successfulJobRuns',
            warningsJobRuns: 'warningsJobRuns',
            failedJobRuns: 'failedJobRuns',
            maxJobDuration: 'maxJobDuration',
            maxDurationBackupJobName: 'maxDurationBackupJobName',
            maxBackupJobDuration: 'maxBackupJobDuration',
            maxDurationReplicaJobName: 'maxDurationReplicaJobName',
            maxReplicaJobDuration: 'maxReplicaJobDuration',
        };
    } else {
        names = {
            runningJobs: 'RunningJobs',
            scheduledJobs: 'ScheduledJobs',
            scheduledBackupJobs: 'ScheduledBackupJobs',
            scheduledReplicaJobs: 'ScheduledReplicaJobs',
            totalJobRuns: 'TotalJobRuns',
            successfulJobRuns: 'SuccessfulJobRuns',
            warningsJobRuns: 'WarningsJobRuns',
            failedJobRuns: 'FailedJobRuns',
            maxJobDuration: 'MaxJobDuration',
            maxDurationBackupJobName: 'MaxDurationBackupJobName',
            maxBackupJobDuration: 'MaxBackupJobDuration',
            maxDurationReplicaJobName: 'MaxDurationReplicaJobName',
            maxReplicaJobDuration: 'MaxReplicaJobDuration',
        }
    }
    return names
}

// names for collector veeam_backup_servers_metrics (veeam_backup_servers_metrics.collector.yml)
function backupServersNames(version) {
    var names;
    if(version >= 13) {
        names = {
            backupServers: 'backupServers',
            name : 'name',
            description : 'description',
            port : 'port',
            version : 'version',
        };
    } else {
        names = {
            backupServers: 'BackupServers',
            name : 'Name',
            description : 'Description',
            port : 'Port',
            version : 'Version',
        }
    }
    return names
}

// names for collector veeam_backup_jobs_sessions_metrics (veeam_backup_jobs_sessions_metrics.collector.yml)
function jobSessionNames(version) {
    var names;
    if(version >= 13) {
        names = {
            // for loop
            entities: 'entities',
            backupJobSessions: 'backupJobSessions',
            link: 'link',
            name: 'name',
            jobName: 'jobName',
            jobType: 'jobType',
            creationTimeUTC: 'creationTimeUTC',
            endTimeUTC: 'endTimeUTC',
            progress: 'progress',
            uid: 'uid',
            result: 'result',
            state: 'state',
        };
    } else {
        names = {
            // for loop
            entities: 'Entities',
            backupJobSessions: 'BackupJobSessions',

            link: 'Link',
            name: 'Name',
            jobName: 'JobName',
            jobType: 'JobType',
            creationTimeUTC: 'CreationTimeUTC',
            endTimeUTC: 'EndTimeUTC',
            progress: 'Progress',
            uid: 'UID',
            result: 'Result',
            state: 'State',
        }
    }
    return names
}

// names for collector veeam_backup_jobs_tasks_sessions_metrics (veeam_backup_jobs_tasks_sessions_metrics.collector.yml)
function taskSessionNames(version) {
    var names;
    if(version >= 13) {
        names = {
            // for loop
            entities: 'entities',
            backupJobSessions: 'backupTaskSessions',

            links: 'links',
            type: 'type',

            name: 'name',
            creationTimeUTC: 'creationTimeUTC',
            endTimeUTC: 'endTimeUTC',
            uid: 'uid',
            result: 'Result',
            state: 'State',
            vmDisplayName: 'vmDisplayName',
            totalSize: 'totalSize',
            reason: 'reason',
        };
    } else {
        names = {
            // for loop
            entities: 'Entities',
            backupJobSessions: 'BackupTaskSessions',

            links: 'Links',
            type: 'Type',

            name: 'Name',
            creationTimeUTC: 'CreationTimeUTC',
            endTimeUTC: 'EndTimeUTC',
            uid: 'UID',
            result: 'Result',
            state: 'State',
            vmDisplayName: 'VmDisplayName',
            totalSize: 'TotalSize',
            reason: 'Reason',
        }
    }
    return names
}

// Export functions for use in YAML configuration files
module.exports = {
    veeamNames: veeamNames,
    vmNames: vmNames,
    agentsNames: agentsNames,
    jobStatisticsNames: jobStatisticsNames,
    repositoriesNames: repositoriesNames,
    backupServersNames: backupServersNames,
    jobSessionNames: jobSessionNames,
    taskSessionNames: taskSessionNames,

    backupAgentJobStatus: backupAgentJobStatus,
    veeamVersion: veeamVersion,
    currentElementStatus: currentElementStatus,
    jobStatus: jobStatus,
    buildJobs: buildJobs,
    taskStatus: taskStatus,
    buildTasks: buildTasks,
}

