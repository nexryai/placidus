// jsdom lacks AnimationEvent; React checks for it when registering animation handlers.
window.AnimationEvent = Event as unknown as typeof AnimationEvent;
