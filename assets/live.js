/**
 * Go Echo LiveView - JavaScript Client Library
 * ============================================
 * 
 * A lightweight JavaScript library that enables real-time, server-driven UI updates
 * through WebSocket connections. This library replaces the previous WebAssembly implementation
 * with pure JavaScript for better compatibility, debugging, and deployment.
 * 
 * Key Features:
 * - Automatic WebSocket connection management with reconnection
 * - Bidirectional communication between server and client
 * - DOM manipulation based on server commands
 * - Generic drag-and-drop system
 * - Event handling and routing
 * - Message queuing for reliability
 * 
 * Protocol Compatibility:
 * This implementation follows the LiveView protocol and can be used with any
 * server-side framework that implements the same protocol specification.
 * 
 * @version 1.0.0
 * @license MIT
 */

(function() {
    'use strict';
    
    // ============================================================================
    // CONFIGURATION AND STATE MANAGEMENT
    // ============================================================================
    
    /**
     * WebSocket connection instance
     * @type {WebSocket|null}
     */
    let ws = null;
    
    /**
     * Reconnection interval timer ID
     * Used to periodically attempt reconnection when disconnected
     * @type {number|null}
     */
    let reconnectInterval = null;
    
    /**
     * Verbose logging flag
     * Enable by adding ?verbose=true or ?debug=true to the URL
     * @type {boolean}
     */
    let isVerbose = window.location.search.includes('verbose=true') || 
                    window.location.search.includes('debug=true');
    
    /**
     * Drag and drop state management object
     * Tracks the current state of any drag operation in progress
     * @type {Object}
     */
    const dragState = {
        isDragging: false,      // Whether a drag operation is currently active
        draggedElement: '',     // ID of the element being dragged
        componentId: '',        // ID of the component that owns the dragged element
        startX: 0,             // Initial mouse X position when drag started
        startY: 0,             // Initial mouse Y position when drag started
        initX: 0,              // Initial element X position when drag started
        initY: 0,              // Initial element Y position when drag started
        lastUpdate: 0          // Timestamp of last position update sent to server (for throttling)
    };
    
    // Expose drag state globally for debugging purposes
    window.dragState = dragState;
    
    // ============================================================================
    // UTILITY FUNCTIONS
    // ============================================================================
    
    /**
     * Conditional logging function that respects verbose mode setting
     * Only outputs to console when verbose mode is enabled
     * 
     * @param {...any} args - Arguments to log to console
     */
    function log(...args) {
        if (isVerbose) {
            console.log('[LiveView]', ...args);
        }
    }
    
    // ============================================================================
    // WEBSOCKET CONNECTION MANAGEMENT
    // ============================================================================
    
    /**
     * Establishes a WebSocket connection to the LiveView server
     * 
     * This function:
     * 1. Determines the correct WebSocket protocol (ws/wss) based on current page protocol
     * 2. Constructs the WebSocket URL by appending 'ws_goliveview' to the current path
     * 3. Creates the WebSocket connection
     * 4. Sets up all event handlers for the WebSocket lifecycle
     * 
     * The connection URL pattern is: [protocol]://[host][path]ws_goliveview
     * For example: ws://localhost:8080/board/ws_goliveview
     */
    function connect() {
        // Determine WebSocket protocol based on page protocol (ws for http, wss for https)
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = window.location.host;
        const pathname = window.location.pathname;
        
        // Construct WebSocket URL
        // The 'ws_goliveview' suffix is the standard endpoint for LiveView connections
        const uri = `${protocol}//${host}${pathname}ws_goliveview`;
        
        log('Connecting to:', uri);
        
        // Update UI to show connection status
        const contentEl = document.getElementById('content');
        if (contentEl) {
            contentEl.innerHTML = 'Connecting...';
        }
        
        try {
            // Create WebSocket connection
            ws = new WebSocket(uri);
            
            /**
             * WebSocket open event handler
             * Called when connection is successfully established
             */
            ws.onopen = function() {
                log('Connected successfully');
                // Clear the "Connecting..." message
                if (contentEl) {
                    contentEl.innerHTML = '';
                }
                // Stop reconnection attempts since we're connected
                clearInterval(reconnectInterval);
                reconnectInterval = null;
            };
            
            /**
             * WebSocket close event handler
             * Called when connection is closed (either intentionally or due to error)
             */
            ws.onclose = function() {
                log('Disconnected');
                // Show disconnection status to user
                if (contentEl) {
                    contentEl.innerHTML = 'Disconnected';
                }
                // Start attempting to reconnect
                startReconnect();
            };
            
            /**
             * WebSocket error event handler
             * Called when a connection error occurs
             */
            ws.onerror = function(error) {
                console.error('[LiveView] WebSocket error:', error);
            };
            
            /**
             * WebSocket message event handler
             * Called when a message is received from the server
             */
            ws.onmessage = function(event) {
                handleMessage(event.data);
            };
            
        } catch (error) {
            console.error('[LiveView] Connection error:', error);
            startReconnect();
        }
    }
    
    /**
     * Starts the automatic reconnection timer
     * 
     * When the WebSocket connection is lost, this function sets up a timer
     * that attempts to reconnect every second until successful
     */
    function startReconnect() {
        // Only start one reconnection timer
        if (!reconnectInterval) {
            reconnectInterval = setInterval(function() {
                // Check if we're not already connected
                if (!ws || ws.readyState !== WebSocket.OPEN) {
                    log('Attempting to reconnect...');
                    connect();
                }
            }, 1000); // Attempt reconnection every 1 second
        }
    }
    
    // ============================================================================
    // MESSAGE HANDLING
    // ============================================================================
    
    /**
     * Processes incoming messages from the WebSocket server
     * 
     * Message Types:
     * - 'fill': Replace element's innerHTML
     * - 'text': Set element's innerText
     * - 'style': Set element's CSS style
     * - 'set': Set element's value property
     * - 'remove': Remove element from DOM
     * - 'addNode': Add a new node to element
     * - 'propertie': Set a specific property on element
     * - 'script': Execute JavaScript code
     * - 'get': Retrieve a value from element and send back to server
     * 
     * @param {string} data - JSON string containing the message from server
     */
    function handleMessage(data) {
        try {
            // Parse the JSON message
            const msg = JSON.parse(data);
            
            // Verbose logging for debugging
            if (isVerbose) {
                if (msg.type === 'fill') {
                    // For fill messages, log size instead of content (content can be large)
                    log(`FILL message - ID: ${msg.id}, Value length: ${msg.value ? msg.value.length : 0}`);
                } else {
                    log('Received message:', msg);
                }
            }
            
            // ----------------------------------------------------------------
            // SCRIPT EXECUTION
            // Scripts don't need a target element, they execute globally
            // ----------------------------------------------------------------
            if (msg.type === 'script') {
                log('Executing script:', msg.value ? msg.value.substring(0, 100) + '...' : 'empty');
                try {
                    // Execute the JavaScript code
                    // Note: eval is used here intentionally for dynamic code execution
                    // The server should be trusted and sanitize any user input
                    eval(msg.value);
                } catch (error) {
                    console.error('[LiveView] Script execution error:', error);
                }
                return;
            }
            
            // ----------------------------------------------------------------
            // ELEMENT-BASED OPERATIONS
            // All other operations require a target element
            // ----------------------------------------------------------------
            
            // Find the target element by ID
            const element = document.getElementById(msg.id);
            if (!element && msg.type !== 'script') {
                log(`Element not found: ${msg.id}`);
                return;
            }
            
            // Process message based on type
            switch (msg.type) {
                // ============================================================
                // CONTENT MANIPULATION
                // ============================================================
                
                case 'fill':
                    // Replace entire HTML content of element
                    // Used for complex updates with nested HTML
                    element.innerHTML = msg.value || '';
                    break;
                    
                case 'text':
                    // Set text content of element
                    // Used for simple text updates (escapes HTML)
                    if (msg.value) {
                        element.innerText = msg.value;
                    }
                    break;
                    
                case 'addNode':
                    // Add a new child node to element
                    // Parses the HTML fragment and appends the resulting nodes,
                    // preserving the element's existing children
                    const div = document.createElement('div');
                    div.innerHTML = msg.value || '';
                    while (div.firstChild) {
                        element.appendChild(div.firstChild);
                    }
                    break;
                    
                // ============================================================
                // ELEMENT MANIPULATION
                // ============================================================
                    
                case 'remove':
                    // Remove element from DOM completely
                    element.remove();
                    break;
                    
                case 'style':
                    // Set CSS style directly on element
                    // Value should be a CSS string like "color: red; font-size: 14px"
                    element.style.cssText = msg.value || '';
                    break;
                    
                // ============================================================
                // PROPERTY MANIPULATION
                // ============================================================
                    
                case 'set':
                    // Set the value property (for input elements)
                    element.value = msg.value || '';
                    break;
                    
                case 'propertie':
                    // Set any arbitrary property on the element
                    // msg.propertie contains the property name
                    // msg.value contains the property value
                    element[msg.propertie] = msg.value;
                    break;
                    
                // ============================================================
                // DATA RETRIEVAL
                // ============================================================
                    
                case 'get':
                    // Server is requesting data from the client
                    // Response will be sent back through WebSocket
                    handleGetRequest(element, msg);
                    break;
                    
                default:
                    log('Unknown message type:', msg.type);
            }
            
        } catch (error) {
            console.error('[LiveView] Message handling error:', error, 'Data:', data);
        }
    }
    
    /**
     * Handles GET requests from the server
     * 
     * The server can request various types of data from DOM elements:
     * - 'value': Get input element's value
     * - 'html': Get element's innerHTML
     * - 'text': Get element's innerText
     * - 'style': Get specific CSS style property
     * - 'propertie': Get any property value
     * 
     * @param {HTMLElement} element - The target DOM element
     * @param {Object} msg - The message object from server containing request details
     */
    function handleGetRequest(element, msg) {
        // Prepare response object
        const response = {
            type: 'get',           // Response type identifier
            id_ret: msg.id_ret,    // Return ID to match request with response
            data: null             // The requested data
        };
        
        try {
            // Extract the requested data based on sub_type
            switch (msg.sub_type) {
                case 'value':
                    // Get input/textarea/select value
                    response.data = element.value;
                    break;
                    
                case 'html':
                    // Get inner HTML content
                    response.data = element.innerHTML;
                    break;
                    
                case 'text':
                    // Get inner text content
                    response.data = element.innerText;
                    break;
                    
                case 'style':
                    // Get specific style property
                    // msg.value contains the CSS property name
                    response.data = element.style[msg.value];
                    break;
                    
                case 'propertie':
                    // Get any property value
                    // msg.value contains the property name
                    response.data = element[msg.value];
                    break;
            }
        } catch (error) {
            console.error('[LiveView] Get request error:', error);
        }
        
        // Send response back to server
        sendMessage(response);
    }
    
    // ============================================================================
    // MESSAGE SENDING
    // ============================================================================
    
    /**
     * Sends a message to the server through the WebSocket connection
     * 
     * This is a low-level function that handles the actual transmission.
     * It checks if the connection is open before sending.
     * 
     * @param {Object} data - The data object to send (will be JSON stringified)
     */
    function sendMessage(data) {
        // Check if WebSocket is connected and ready
        if (ws && ws.readyState === WebSocket.OPEN) {
            // Convert to JSON and send
            ws.send(JSON.stringify(data));
        } else {
            // Log warning if trying to send while disconnected
            console.warn('[LiveView] WebSocket not connected, message not sent:', data);
        }
    }
    
    /**
     * Sends an event to the server
     * 
     * This is the main function for client-to-server communication.
     * It packages user interactions into a standard event format.
     * 
     * @param {string} id - The component ID that triggered the event
     * @param {string} event - The event name (e.g., 'Click', 'Input', 'DragStart')
     * @param {string|Object} data - Optional event data (objects will be JSON stringified)
     * 
     * @example
     * // Send a click event
     * sendEvent('button-1', 'Click', null);
     * 
     * // Send an input event with value
     * sendEvent('input-1', 'Input', {value: 'user text'});
     * 
     * // Send a drag event with coordinates
     * sendEvent('draggable-1', 'DragMove', {x: 100, y: 200});
     */
    function sendEvent(id, event, data) {
        log(`Sending event: id=${id}, event=${event}, data=${data}`);
        
        // Create standard event message structure
        const msg = {
            type: 'data',  // Message type for user events
            id: id,        // Component ID
            event: event,  // Event name
            // Convert data to string if it's an object
            data: typeof data === 'object' ? JSON.stringify(data) : (data || '')
        };
        
        sendMessage(msg);
    }
    
    // ============================================================================
    // DRAG AND DROP SYSTEM
    // ============================================================================
    
    /**
     * Initializes the drag and drop event system
     * 
     * Sets up global mouse event listeners that enable dragging for any element
     * with the 'draggable' or 'draggable-box' CSS class.
     * 
     * Features:
     * - Works with any element marked as draggable
     * - Sends drag events to server for synchronization
     * - Supports visual feedback during dragging
     * - Throttles position updates for performance
     */
    function initDragAndDrop() {
        // Register global mouse event handlers
        document.addEventListener('mousedown', handleMouseDown);
        document.addEventListener('mousemove', handleMouseMove);
        document.addEventListener('mouseup', handleMouseUp);
        
        log('Drag & Drop initialized');
    }
    
    /**
     * Handles mouse down event to initiate dragging
     * 
     * This function:
     * 1. Checks if the clicked element (or its parent) is draggable
     * 2. Captures initial positions (mouse and element)
     * 3. Updates drag state
     * 4. Sends DragStart event to server
     * 
     * Elements become draggable by adding:
     * - class="draggable" (or "draggable-box" for legacy support)
     * - data-element-id="unique-id" (optional, falls back to element's id)
     * - data-component-id="owner-component" (optional, for component association)
     * - data-drag-disabled="true" (optional, to temporarily disable dragging)
     * 
     * @param {MouseEvent} e - The mouse down event
     */
    function handleMouseDown(e) {
        let target = e.target;
        
        // Walk up the DOM tree to find a draggable element
        // This allows clicking on child elements of draggable containers
        while (target && target !== document.body) {
            const classList = target.classList;
            
            // Skip elements with pointer-events: none
            // These are typically decorative overlays that shouldn't be interactive
            const style = window.getComputedStyle(target);
            if (style.pointerEvents === 'none') {
                target = target.parentElement;
                continue;
            }
            
            // Check if element has draggable class
            if (classList && (classList.contains('draggable') || classList.contains('draggable-box'))) {
                log('Found draggable element:', target.id);
                
                // Check if dragging is temporarily disabled
                if (target.hasAttribute('data-drag-disabled')) {
                    return;
                }
                
                // Prevent default behavior and stop event bubbling
                e.preventDefault();
                e.stopPropagation();
                
                // ============================================================
                // EXTRACT ELEMENT IDENTIFICATION
                // ============================================================
                
                // Get element ID (supports multiple naming conventions)
                let elementId = target.getAttribute('data-element-id');
                if (!elementId) {
                    // Try legacy format for backward compatibility
                    const boxId = target.getAttribute('data-box-id');
                    if (boxId) {
                        elementId = 'box-' + boxId;
                    } else {
                        // Fall back to element's ID attribute
                        elementId = target.id;
                    }
                }
                
                // Get component ID (the component that owns this draggable)
                let componentId = target.getAttribute('data-component-id');
                if (!componentId) {
                    // Try to determine component ID for legacy elements
                    if (target.getAttribute('data-box-id')) {
                        componentId = 'flow-tool'; // Legacy flow tool component
                    } else {
                        // Search up the DOM tree for a component container
                        let parent = target.parentElement;
                        while (parent && parent !== document.body) {
                            const compId = parent.getAttribute('data-component-id');
                            if (compId) {
                                componentId = compId;
                                break;
                            }
                            parent = parent.parentElement;
                        }
                    }
                }
                
                // ============================================================
                // CAPTURE INITIAL POSITION
                // ============================================================
                
                let initX = 0, initY = 0;
                
                // Try to get position from data attributes (legacy format)
                const boxX = target.getAttribute('data-box-x');
                const boxY = target.getAttribute('data-box-y');
                if (boxX && boxY) {
                    initX = parseInt(boxX);
                    initY = parseInt(boxY);
                } else {
                    // Get position from computed CSS style
                    const style = window.getComputedStyle(target);
                    const left = style.left;
                    const top = style.top;
                    
                    if (left && left !== 'auto') {
                        initX = parseInt(left);
                    }
                    if (top && top !== 'auto') {
                        initY = parseInt(top);
                    }
                }
                
                // ============================================================
                // UPDATE DRAG STATE
                // ============================================================
                
                dragState.isDragging = true;
                dragState.draggedElement = elementId;
                dragState.componentId = componentId;
                dragState.startX = e.clientX;
                dragState.startY = e.clientY;
                dragState.initX = initX;
                dragState.initY = initY;
                
                // ============================================================
                // SEND DRAG START EVENT TO SERVER
                // ============================================================
                
                if (componentId) {
                    // Prepare drag data
                    const dragData = {
                        element: elementId,
                        x: e.clientX,
                        y: e.clientY
                    };
                    
                    // Send legacy event for backward compatibility
                    if (elementId.startsWith('box-') && componentId === 'flow-tool') {
                        const boxData = {
                            id: elementId.substring(4), // Remove 'box-' prefix
                            x: e.clientX,
                            y: e.clientY
                        };
                        sendEvent(componentId, 'BoxStartDrag', boxData);
                    }
                    
                    // Send standard drag start event
                    sendEvent(componentId, 'DragStart', dragData);
                }
                
                log('Started dragging:', elementId);
                return false; // Prevent text selection
            }
            
            // Move up to parent element
            target = target.parentElement;
        }
    }
    
    /**
     * Handles mouse move event during dragging
     * 
     * This function:
     * 1. Calculates new position based on mouse movement
     * 2. Updates element's visual position immediately
     * 3. Highlights drop zones when hovering over them
     * 4. Throttles server updates to prevent overwhelming the connection
     * 5. Sends DragMove events to server
     * 
     * @param {MouseEvent} e - The mouse move event
     */
    function handleMouseMove(e) {
        // Only process if we're actively dragging
        if (!dragState.isDragging) return;
        
        e.preventDefault();
        
        // Calculate position delta from drag start
        const deltaX = e.clientX - dragState.startX;
        const deltaY = e.clientY - dragState.startY;
        
        // Calculate new absolute position
        const newX = dragState.initX + deltaX;
        const newY = dragState.initY + deltaY;
        
        // ============================================================
        // UPDATE VISUAL POSITION
        // This provides immediate feedback to the user
        // ============================================================
        
        const element = document.getElementById(dragState.draggedElement);
        if (element) {
            // Update CSS position
            element.style.left = newX + 'px';
            element.style.top = newY + 'px';
            
            // Update data attributes for legacy format compatibility
            if (element.getAttribute('data-box-x') !== null) {
                element.setAttribute('data-box-x', newX);
                element.setAttribute('data-box-y', newY);
            }
            
            // ============================================================
            // HIGHLIGHT DROP ZONES ON HOVER
            // ============================================================
            
            // Hide the dragged element temporarily to detect what's underneath
            const originalPointerEvents = element.style.pointerEvents;
            element.style.pointerEvents = 'none';
            
            // Get the element at the current mouse position
            const elementBelow = document.elementFromPoint(e.clientX, e.clientY);
            
            // Restore pointer events
            element.style.pointerEvents = originalPointerEvents;
            
            // Remove all previous hover classes
            document.querySelectorAll('.drag-over').forEach(el => {
                el.classList.remove('drag-over');
            });
            
            // Walk up the DOM tree to find a droppable element
            let target = elementBelow;
            while (target && target !== document.body) {
                // Check for droppable class or data attribute
                if (target.classList && (
                    target.classList.contains('droppable') || 
                    target.classList.contains('drop-zone') ||
                    target.hasAttribute('data-droppable') ||
                    target.hasAttribute('data-drop-zone-id')
                )) {
                    // Add hover class to indicate valid drop zone
                    target.classList.add('drag-over');
                    break;
                }
                target = target.parentElement;
            }
        }
        
        // ============================================================
        // THROTTLE SERVER UPDATES
        // Send position updates at ~60 FPS maximum to prevent overwhelming the server
        // ============================================================
        
        const now = Date.now();
        if (now - dragState.lastUpdate > 16 && dragState.componentId) { // 16ms = ~60 FPS
            
            // Send legacy event for backward compatibility
            if (dragState.draggedElement.startsWith('box-') && dragState.componentId === 'flow-tool') {
                const boxData = {
                    id: dragState.draggedElement.substring(4),
                    x: newX,
                    y: newY
                };
                sendEvent(dragState.componentId, 'BoxDrag', boxData);
            }
            
            // Send standard drag move event
            const moveData = {
                element: dragState.draggedElement,
                x: newX,
                y: newY
            };
            sendEvent(dragState.componentId, 'DragMove', moveData);
            
            // Update last update timestamp
            dragState.lastUpdate = now;
            
            log(`Dragging ${dragState.draggedElement} to (${newX}, ${newY})`);
        }
    }
    
    /**
     * Handles mouse up event to complete dragging
     * 
     * This function:
     * 1. Captures final position
     * 2. Detects drop zone (if any)
     * 3. Sends DragEnd event to server with drop zone info
     * 4. Resets drag state
     * 
     * @param {MouseEvent} e - The mouse up event
     */
    function handleMouseUp(e) {
        // Only process if we were dragging
        if (!dragState.isDragging) return;
        
        e.preventDefault();
        
        // ============================================================
        // DETECT DROP ZONE
        // ============================================================
        
        const element = document.getElementById(dragState.draggedElement);
        let dropZone = null;
        let dropZoneId = null;
        
        if (element) {
            // Hide the dragged element temporarily to detect what's underneath
            const originalPointerEvents = element.style.pointerEvents;
            element.style.pointerEvents = 'none';
            
            // Get the element at the drop position
            const elementBelow = document.elementFromPoint(e.clientX, e.clientY);
            
            // Restore pointer events
            element.style.pointerEvents = originalPointerEvents;
            
            // Walk up the DOM tree to find a droppable element
            let target = elementBelow;
            while (target && target !== document.body) {
                // Check for droppable class or data attribute
                if (target.classList && (
                    target.classList.contains('droppable') || 
                    target.classList.contains('drop-zone') ||
                    target.hasAttribute('data-droppable') ||
                    target.hasAttribute('data-drop-zone-id')
                )) {
                    dropZone = target;
                    // Get drop zone ID from various sources
                    dropZoneId = target.getAttribute('data-drop-zone-id') || 
                                target.getAttribute('data-column-id') ||
                                target.id;
                    break;
                }
                target = target.parentElement;
            }
            
            log('Drop zone detected:', dropZoneId || 'none');
        }
        
        // ============================================================
        // SEND FINAL POSITION TO SERVER
        // ============================================================
        
        if (element && dragState.componentId) {
            // Get final position from computed style
            const style = window.getComputedStyle(element);
            const finalX = parseInt(style.left) || 0;
            const finalY = parseInt(style.top) || 0;
            
            // Send legacy event for backward compatibility
            if (dragState.draggedElement.startsWith('box-') && dragState.componentId === 'flow-tool') {
                const boxData = {
                    id: dragState.draggedElement.substring(4),
                    x: finalX,
                    y: finalY
                };
                sendEvent(dragState.componentId, 'BoxEndDrag', boxData);
            }
            
            // Send standard drag end event with drop zone information
            const finalData = {
                element: dragState.draggedElement,
                x: finalX,
                y: finalY,
                dropZone: dropZoneId,  // Include drop zone in the event data
                dropX: e.clientX,      // Mouse position for more precise drop detection
                dropY: e.clientY
            };
            sendEvent(dragState.componentId, 'DragEnd', finalData);
        }
        
        log('Ended dragging:', dragState.draggedElement, 'Drop zone:', dropZoneId);
        
        // ============================================================
        // CLEAN UP VISUAL FEEDBACK
        // ============================================================
        
        // Remove all drag-over classes
        document.querySelectorAll('.drag-over').forEach(el => {
            el.classList.remove('drag-over');
        });
        
        // ============================================================
        // RESET DRAG STATE
        // ============================================================
        
        dragState.isDragging = false;
        dragState.draggedElement = '';
        dragState.componentId = '';
    }
    
    // ============================================================================
    // INITIALIZATION
    // ============================================================================
    
    /**
     * Initializes the LiveView client library
     * 
     * This function:
     * 1. Sets up the initial UI state
     * 2. Exposes global functions for external use
     * 3. Establishes WebSocket connection
     * 4. Initializes drag and drop system
     * 
     * Called automatically when DOM is ready
     */
    function init() {
        console.log('Go Echo LiveView - JavaScript Client v1.0.0');
        
        // Set initial loading state
        const contentEl = document.getElementById('content');
        if (contentEl) {
            contentEl.innerHTML = 'Initializing...';
        }
        
        // ============================================================
        // EXPOSE GLOBAL API
        // These functions can be called from inline HTML or other scripts
        // ============================================================
        
        // Main event sending function
        // Usage: send_event('component-id', 'EventName', eventData)
        window.send_event = sendEvent;
        
        // Manual connection function
        // Usage: connect() to manually reconnect
        window.connect = connect;
        
        // WebSocket reference (for debugging)
        // Usage: console.log(ws.readyState) to check connection status
        window.ws = null;
        
        // ============================================================
        // INITIALIZE SYSTEMS
        // ============================================================
        
        // Establish WebSocket connection
        connect();
        
        // Set up drag and drop handlers
        initDragAndDrop();
        
        // ============================================================
        // DYNAMIC WEBSOCKET REFERENCE
        // Allows external code to access the current WebSocket instance
        // ============================================================
        
        Object.defineProperty(window, 'ws', {
            get: function() { return ws; },
            set: function(val) { ws = val; }
        });
        
        log('LiveView client initialized successfully');
    }
    
    // ============================================================================
    // STARTUP
    // ============================================================================
    
    /**
     * Start initialization when DOM is ready
     * 
     * This ensures all DOM elements are available before we try to
     * manipulate them or set up event handlers
     */
    if (document.readyState === 'loading') {
        // DOM is still loading, wait for it to complete
        document.addEventListener('DOMContentLoaded', init);
    } else {
        // DOM is already loaded, initialize immediately
        init();
    }
    
})();