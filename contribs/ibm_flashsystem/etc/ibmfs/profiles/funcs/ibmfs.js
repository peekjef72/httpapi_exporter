// IBM FlashSytem status mapping to numeric values for Prometheus
function globalStatus(status) {
    var val = 3;
    status = status.toLowerCase();
    switch (status) {
        case "degraded":
            val = 2;
            break;
        case "offline":
            val = 0;
            break;
        case "online":
            val = 1;
            break;
        default:
            val = 3;
            console.log("unknown status " + status);
            break;
    }
    return val;
}

// IBM port Fiber Channel status mapping to numeric values for Prometheus
function portFCStatus(status) {
    var val = 3; // unknown
    status = status.toLowerCase();
    switch (status) {
        case 'active':
            val = 1;
            break;
        case "inactive_configured":
            val = 0;
            break;
        case "inactive_unconfigured":
            val = 2
        default:
            val = 3;
            console.log("unknown FC port status " + status);
            break;
    }
    return val;
}

// IBM port ethernet status mapping to numeric values for Prometheus
function portStatus(status) {
    var val = 2; // unknown
    status = status.toLowerCase();
    switch (status) {
        case 'active':
            val = 1;
            break;
        case "inactive":
            val = 0;
            break;
        default:
            val = 2;
            console.log("unknown ethernet port status " + status);
            break;
    }
    return val;
}

// IBM port Fiber Channel status mapping to numeric values for Prometheus
// received  "1Gb\/s"
function ethernetSpeed(speed_str) {
    var speed = 0;
    // console.log("received speed " + speed_str);
    if ( speed_str == '' ) {
        return speed;
    }
    elmt = speed_str.split("\/")
    if( elmt.length > 0 ) {
        // console.debug("elmt[0]:" + elmt[0] );
        const match = elmt[0].match(/(\d+(?:\.\d+)?)\s*([a-zA-Z]+)/);
        if (!match) {
            console.warn(" can't extract value and unit from string!")
            return speed;
        }
        // console.debug("match[1,2]:", match[1], match[2]);
        speed = parseFloat(match[1]);
        const unit = match[2].replace(/[^a-zA-Z]/g, '');
        switch (unit) {
            case "kilobit":
            case "Kb":
                speed = speed * 1000
                break;
            case "megabit":
            case "Mb":
                speed = speed * 1000 * 1000
                break;
            case "gigabit":
            case "Gb":
                speed = speed * 1000 * 1000 * 1000
                break;
        
        }
    }
    return speed;
}

// IBM port ethernet status mapping to numeric values for Prometheus
function ethernetDuplex(status) {
    // console.log("received duplex type " + duplex);
    status = status.toLowerCase();
    var val = 0; // unknown
    switch (status) {
        case 'full':
            val = 2;
            break;
        case "half":
            val = 1;
            break;
        default:
            val = 0;
            console.log("unknown duplex type " + status);
            break;
    }
    return val;
}

// IBM array/disk size string to numeric values for Prometheus
// received  "1Gb\/s"
function elementSizeBytes(size_str) {
    var size = -1;
    // console.log("received size " + size_str);
    if ( !size_str || size_str != '' ) {
        var match;
        try {
            match = size_str.match(/(\d+(?:\.\d+)?)\s*([a-zA-Z]+)/);
        }
        catch( error ) {
            console.error(error);
        }
        if (!match) {
            console.debug(" can't extract value and unit from string!")
        } else if( match.length >= 2 ) {
            size = exporter.convertToBytes(match[1], match[2])
        }
    }
    return size;
}

// IBM partition HA status mapping to numeric values for Prometheus
// from gemini ha_status available values are : 
// null or empty set to 0
// encountered_a_problem => 4
// suspended => 3
// establishing => 2
// established => 1
function HAStatus(status) {
    // console.log("received ha status type " + status);
    var val = 0; // unknown
    status = status.toLowerCase();
    switch (status) {
        case 'encountered_a_problem':
            val = 4;
            break;
        case 'suspended':
            val = 3;
            break;
        case "establishing":
            val = 2;
            break;
        case "established":
            val = 1;
            break;
        default:
            val = 0;
            break;
    }
    return val;
}

// IBM partition link status mapping to numeric values for Prometheus
// from gemini link_status available values are :
// null or empty set to 0
// synchronized => 1
// synchronizing => 2
// not_synchronized => 3
function linkStatus(status) {
    // console.log("received link status type " + status);
    var val = 0; // unknown
    status = status.toLowerCase();
    switch (status) {
        case "synchronized":
            val = 1;
            break;
        case 'synchronizing':
            val = 2;
            break;
        case "not_synchronized":
            val = 3;
            break;
        default:
            val = 0;
            break;
    }
    return val;
}

// *******************************************************************
// Export functions for use in YAML configuration files
module.exports = {
    globalStatus: globalStatus,
    portFCStatus: portFCStatus,
    portStatus: portStatus,
    ethernetDuplex: ethernetDuplex,
    ethernetSpeed: ethernetSpeed,
    elementSizeBytes: elementSizeBytes,
    HAStatus: HAStatus,
    linkStatus: linkStatus,
}
