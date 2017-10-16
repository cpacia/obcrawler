//http://crawler.ob1.io/charts/nodes?timeFrame=48h

// Wait for DOM to load
document.addEventListener("DOMContentLoaded", function(event) {
    let oldTimeFrame = "7dTimeFrame";
    let oldLastActive = "7dLastActive";
    let oldLastNodeType = "allNodes";

    let timeFrameUrl = lastActiveUrl = "168h";

    let nodeType = "";

    // Define base chart URL
    let baseImageUrl = `${document.getElementById("chartImage").src.split("?")[0]}?`;

    // Define list of doc ID's
    const allTimeFrames = {
        "1hTimeFrame": "1h",
        "12hTimeFrame": "12h",
        "1dTimeFrame": "24h",
        "3dTimeFrame": "72h",
        "7dTimeFrame": "168h",
        "1mTimeFrame": "720h",
        "3mTimeFrame": "2160h",
        "1yTimeFrame": "8760h",
        "allTimeFrame": "99999h"
    };

    const allLastActiveTimes = {
        "1hLastActive": "1h",
        "12hLastActive": "12h",
        "1dLastActive": "24h",
        "3dLastActive": "72h",
        "7dLastActive": "168h",
        "1mLastActive": "720h",
        "3mLastActive": "2160h",
        "1yLastActive": "8760h",
        "allLastActive": "99999h"
    };

    const allNodeTypes = {
        "clearnetNodes": "clearnet",
        "torNodes": "tor",
        "dualstackNodes": "dualstack",
        "allNodes": ""
    };

    // Setup events for clicking on timeframe 
    for (let i = 0; i < Object.keys(allTimeFrames).length; i++) {
        document.getElementById(Object.keys(allTimeFrames)[i]).onclick = function() {
            timeFrameUrl = allTimeFrames[Object.keys(allTimeFrames)[i]]

            // Update currently selected node time frame
            document.getElementById(oldTimeFrame).style.textDecoration = "none";
            document.getElementById(oldTimeFrame).style.fontWeight = "normal";
            document.getElementById(Object.keys(allTimeFrames)[i]).style.textDecoration = "underline";
            document.getElementById(Object.keys(allTimeFrames)[i]).style.fontWeight = "bold";

            oldTimeFrame = Object.keys(allTimeFrames)[i];

            changeChartImage();
        }
    };

    // Setup events for clicking on last active times
    for (let i = 0; i < Object.keys(allLastActiveTimes).length; i++) {
        document.getElementById(Object.keys(allLastActiveTimes)[i]).onclick = function() {
            lastActiveUrl = allLastActiveTimes[Object.keys(allLastActiveTimes)[i]]
            // Update currently selected node last active
            document.getElementById(oldLastActive).style.textDecoration = "none";
            document.getElementById(oldLastActive).style.fontWeight = "normal";
            document.getElementById(Object.keys(allLastActiveTimes)[i]).style.textDecoration = "underline";
            document.getElementById(Object.keys(allLastActiveTimes)[i]).style.fontWeight = "bold";

            oldLastActive = Object.keys(allLastActiveTimes)[i];
            changeChartImage();
        }
    };

    if (document.getElementById("nodesOnline")) {
        // Setup events for clicking on type of node 
        for (let i = 0; i < Object.keys(allNodeTypes).length; i++) {
            document.getElementById(Object.keys(allNodeTypes)[i]).onclick = function() {
                nodeType = allNodeTypes[Object.keys(allNodeTypes)[i]]
                // Update currently selected node type
                document.getElementById(oldLastNodeType).style.textDecoration = "none";
                document.getElementById(oldLastNodeType).style.fontWeight = "normal";
                document.getElementById(Object.keys(allNodeTypes)[i]).style.textDecoration = "underline";
                document.getElementById(Object.keys(allNodeTypes)[i]).style.fontWeight = "bold";

                oldLastNodeType = Object.keys(allNodeTypes)[i];
                changeChartImage();
            }
        };
    }

    // Change current chart image
    let changeChartImage = () => {
        let chartImageUrl = `${baseImageUrl}&timeFrame=${timeFrameUrl}&lastActive=${lastActiveUrl}`

        // If on node page then add only in to the request field
        if (document.getElementById("nodesOnline")) {
        	chartImageUrl = `${chartImageUrl}&only=${nodeType}`;
       	}

        // Set image URL to new chart data
        document.getElementById("chartImage").src = chartImageUrl;
    };

});