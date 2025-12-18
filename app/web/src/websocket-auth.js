/**
 * WebSocket Authentication Module for Nostr Relays
 * Implements NIP-42 authentication with proper challenge handling
 */

export class NostrWebSocketAuth {
    constructor(relayUrl, userSigner, userPubkey) {
        this.relayUrl = relayUrl;
        this.userSigner = userSigner;
        this.userPubkey = userPubkey;
        this.ws = null;
        this.challenge = null;
        this.isAuthenticated = false;
        this.authPromise = null;
    }

    /**
     * Connect to relay and handle authentication
     */
    async connect() {
        return new Promise((resolve, reject) => {
            this.ws = new WebSocket(this.relayUrl);
            
            this.ws.onopen = () => {
                console.log('WebSocket connected to relay:', this.relayUrl);
                resolve();
            };
            
            this.ws.onmessage = async (message) => {
                try {
                    const data = JSON.parse(message.data);
                    await this.handleMessage(data);
                } catch (error) {
                    console.error('Error parsing relay message:', error);
                }
            };
            
            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error);
                reject(new Error('Failed to connect to relay'));
            };
            
            this.ws.onclose = () => {
                console.log('WebSocket connection closed');
                this.isAuthenticated = false;
                this.challenge = null;
            };
            
            // Timeout for connection
            setTimeout(() => {
                if (this.ws.readyState !== WebSocket.OPEN) {
                    reject(new Error('Connection timeout'));
                }
            }, 10000);
        });
    }

    /**
     * Handle incoming messages from relay
     */
    async handleMessage(data) {
        const [messageType, ...params] = data;
        
        switch (messageType) {
            case 'AUTH':
                // Relay sent authentication challenge
                this.challenge = params[0];
                console.log('Received AUTH challenge:', this.challenge);
                await this.authenticate();
                break;
                
            case 'OK':
                const [eventId, success, reason] = params;
                if (eventId && success) {
                    console.log('Authentication successful for event:', eventId);
                    this.isAuthenticated = true;
                    if (this.authPromise) {
                        this.authPromise.resolve();
                        this.authPromise = null;
                    }
                } else if (eventId && !success) {
                    console.error('Authentication failed:', reason);
                    if (this.authPromise) {
                        this.authPromise.reject(new Error(reason || 'Authentication failed'));
                        this.authPromise = null;
                    }
                }
                break;
                
            case 'NOTICE':
                console.log('Relay notice:', params[0]);
                break;
                
            default:
                console.log('Unhandled message type:', messageType, params);
        }
    }

    /**
     * Authenticate with the relay using NIP-42
     */
    async authenticate() {
        if (!this.challenge) {
            throw new Error('No challenge received from relay');
        }
        
        if (!this.userSigner) {
            throw new Error('No signer available for authentication');
        }
        
        try {
            // Create NIP-42 authentication event
            const authEvent = {
                kind: 22242, // ClientAuthentication kind
                created_at: Math.floor(Date.now() / 1000),
                tags: [
                    ['relay', this.relayUrl],
                    ['challenge', this.challenge]
                ],
                content: '',
                pubkey: this.userPubkey
            };
            
            // Sign the authentication event
            const signedAuthEvent = await this.userSigner.signEvent(authEvent);
            
            // Send AUTH message to relay
            const authMessage = ["AUTH", signedAuthEvent];
            this.ws.send(JSON.stringify(authMessage));
            
            console.log('Sent authentication event to relay');
            
            // Wait for authentication response
            return new Promise((resolve, reject) => {
                this.authPromise = { resolve, reject };
                
                // Timeout for authentication
                setTimeout(() => {
                    if (this.authPromise) {
                        this.authPromise.reject(new Error('Authentication timeout'));
                        this.authPromise = null;
                    }
                }, 10000);
            });
            
        } catch (error) {
            console.error('Authentication error:', error);
            throw error;
        }
    }

    /**
     * Publish an event to the relay
     */
    async publishEvent(event) {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
            throw new Error('WebSocket not connected');
        }
        
        return new Promise((resolve, reject) => {
            // Send EVENT message
            const eventMessage = ["EVENT", event];
            this.ws.send(JSON.stringify(eventMessage));
            
            // Set up message handler for this specific event
            const originalOnMessage = this.ws.onmessage;
            const timeout = setTimeout(() => {
                this.ws.onmessage = originalOnMessage;
                reject(new Error('Publish timeout'));
            }, 15000);
            
            this.ws.onmessage = async (message) => {
                try {
                    const data = JSON.parse(message.data);
                    const [messageType, eventId, success, reason] = data;
                    
                    if (messageType === 'OK' && eventId === event.id) {
                        if (success) {
                            clearTimeout(timeout);
                            this.ws.onmessage = originalOnMessage;
                            console.log('Event published successfully:', eventId);
                            resolve({ success: true, eventId, reason });
                        } else {
                            console.error('Event publish failed:', reason);

                            // Check if authentication is required
                            if (reason && reason.includes('auth-required')) {
                                console.log('Authentication required, waiting for AUTH challenge...');
                                // Don't restore original handler yet - we need to receive the AUTH challenge
                                // The AUTH message will be handled by the else branch below
                                return;
                            }

                            clearTimeout(timeout);
                            this.ws.onmessage = originalOnMessage;
                            reject(new Error(`Publish failed: ${reason}`));
                        }
                    } else if (messageType === 'AUTH') {
                        // Handle AUTH challenge during publish flow
                        this.challenge = data[1];
                        console.log('Received AUTH challenge during publish:', this.challenge);

                        try {
                            await this.authenticate();
                            console.log('Authentication successful, retrying event publish...');
                            // Re-send the event after authentication
                            const retryMessage = ["EVENT", event];
                            this.ws.send(JSON.stringify(retryMessage));
                            // Don't resolve yet, wait for the retry response
                        } catch (authError) {
                            clearTimeout(timeout);
                            this.ws.onmessage = originalOnMessage;
                            reject(new Error(`Authentication failed: ${authError.message}`));
                        }
                    } else {
                        // Handle other messages normally
                        await this.handleMessage(data);
                    }
                } catch (error) {
                    clearTimeout(timeout);
                    this.ws.onmessage = originalOnMessage;
                    reject(error);
                }
            };
        });
    }

    /**
     * Close the WebSocket connection
     */
    close() {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        this.isAuthenticated = false;
        this.challenge = null;
    }

    /**
     * Check if currently authenticated
     */
    getAuthenticated() {
        return this.isAuthenticated;
    }
}

/**
 * Convenience function to publish an event with authentication
 */
export async function publishEventWithAuth(relayUrl, event, userSigner, userPubkey) {
    const auth = new NostrWebSocketAuth(relayUrl, userSigner, userPubkey);
    
    try {
        await auth.connect();
        const result = await auth.publishEvent(event);
        return result;
    } finally {
        auth.close();
    }
}
