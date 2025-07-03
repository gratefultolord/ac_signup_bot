INSERT INTO admins (chat_id) VALUES (166018759), (320522635);

INSERT INTO categories (title, photo_path, created_at, updated_at)
VALUES
    ('Еда', 'food.png', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('Продукты', 'groceries.png', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('Красота', 'beauty.png', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('Здоровье', 'health.png', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('Путешествия', 'travel.png', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('Другое', 'other.png', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

INSERT INTO partners (category_id, title, description, address, url, photo_path, discount_type, discount_percent_size, created_at, updated_at)
VALUES
    (1, 'Кинза и базилик', 'Аутентичные блюда: приготовленные с мастерством и знанием дела, уют и гостеприимство, радость душевных встреч и благодарные отзывы любимых гостей', 'Мельковская ул. 2Д', 'kinzabazilik.ru', 'kinza_bazilik.png', 'percent', 15, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (1, 'Lo Vegan', 'Современное веган-кафе с авторским подходом к полезной еде и уютной атмосферой. Отличный выбор для тех, кто заботится о себе.', 'ул. Добролюбова, 19', 'lovegan.ru', 'lo_vegan.png', 'percent', 10, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (1, 'Chocoroom', 'Арт-кофейня и десерт-бар, где шоколад становится искусством. Ручная работа, премиальные ингредиенты и стильный интерьер.', 'ул. Большая Покровская, 45', 'chocoroom.ru', 'chocoroom.png', 'percent', 12, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (1, 'Pankoff Bakery', 'Семейная пекарня с настоящей душой: хрустящий хлеб, нежные булочки и кофе, который согревает. Свежее каждый день.', 'ул. Октябрьская, 8', 'pankoffbakery.ru', 'pankoff_bakery.png', 'percent', 8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);