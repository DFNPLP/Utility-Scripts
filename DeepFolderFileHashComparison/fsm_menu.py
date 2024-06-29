class Fsm:

    def __init__(self):
        self._states_and_transitions = {}
        self._current_state = None
        self._data_to_pass_to_state = None

    def _state_exists(self, state_name):
        return state_name in self._states_and_transitions

    def register_states(self, states_set):
        for state in states_set:
            self._states_and_transitions[state] = None

    def set_state(self, state_name, data_to_pass=None):
        if not self._state_exists(state_name):
            raise ValueError(f"{state_name} is not a defined state!")
        else:
            self._current_state = state_name
            self._data_to_pass_to_state = data_to_pass

    def set_state_handler(self, state_name, handler):
        """
        Handler should return <next state>, <data for next state>. <next state> should be None if it
        is a terminal state.
        :param state_name:
        :param handler:
        :return:
        """
        if not self._state_exists(state_name):
            raise ValueError(f"{state_name} is not a defined state!")
        elif not callable(handler):
            raise ValueError(f"Provided handler is not callable!")

        self._states_and_transitions[state_name] = handler

    def execute_fsm(self):
        while self._current_state is not None:
            self._current_state, self._data_to_pass_to_state = \
                self._states_and_transitions[self._current_state](self._data_to_pass_to_state)

        return self._data_to_pass_to_state
