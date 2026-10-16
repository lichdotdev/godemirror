class Ring:
    def __init__(self, capacity: int = 8, *items, name=DEFAULT, **options):
        self.capacity = capacity

    def push(self, item, /, *, at=None):
        each = lambda x, y=2: x + y
        return len(self.items) + each(item)
